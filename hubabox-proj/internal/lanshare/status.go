package lanshare

import (
	"database/sql"
	"path/filepath"
	"strings"
)

// BuildStatus probes the OS and merges KV desired state for the admin UI.
func BuildStatus(db *sql.DB, dataDir, filesDir, hostname string, lanIPs []string) Status {
	shareName := ReadShareName(db)
	desired := ReadDesired(db)
	filesAbs, _ := filepath.Abs(filesDir)
	st := Status{
		Desired:      desired,
		ShareName:    shareName,
		FilesPath:    filesAbs,
		PlatformNote: platformNote(),
	}
	st.UNCPaths = UNCPathsFor(hostname, shareName, lanIPs)
	active, note := probeOS(filesAbs, shareName)
	st.Active = active
	if note != "" {
		st.PlatformNote = note
	}
	linux, win, err := WriteHelperScripts(dataDir, filesAbs, shareName)
	if err == nil {
		st.ScriptLinux = linux
		st.ScriptWin = win
	}
	if desired && !active {
		st.ApplyHint = "Share is requested but not active on the OS yet — click Enable or run the helper script as Administrator/root."
	} else if desired && active {
		st.ApplyHint = "LAN file sharing is active. Guests can use the UNC paths below (read-only)."
	} else if !desired && active {
		st.ApplyHint = "A share exists on this machine; click Disable if you want to remove it."
	}
	return st
}

// Apply tries to create the share on Windows; on Linux it refreshes scripts and re-probes.
func Apply(db *sql.DB, dataDir, filesDir, hostname string, lanIPs []string) Status {
	shareName := ReadShareName(db)
	_ = SetDesired(db, true)
	applied, errMsg := tryApplyOS(filesDir, shareName)
	if errMsg != "" {
		_ = setLastApplyErr(db, errMsg)
	} else {
		_ = setLastApplyErr(db, "")
	}
	st := BuildStatus(db, dataDir, filesDir, hostname, lanIPs)
	if !applied && errMsg != "" {
		st.LastApplyErr = errMsg
		if st.ApplyHint == "" {
			st.ApplyHint = errMsg
		}
	}
	return st
}

// Disable removes desired flag and attempts OS teardown.
func Disable(db *sql.DB, dataDir, filesDir, hostname string, lanIPs []string) Status {
	shareName := ReadShareName(db)
	_, _ = tryRemoveOS(shareName)
	_ = SetDesired(db, false)
	_ = setLastApplyErr(db, "")
	return BuildStatus(db, dataDir, filesDir, hostname, lanIPs)
}

func setLastApplyErr(db *sql.DB, msg string) error {
	if db == nil {
		return nil
	}
	const key = "lanshare_last_err"
	msg = strings.TrimSpace(msg)
	if msg == "" {
		_, err := db.Exec(`DELETE FROM kv WHERE key = ?`, key)
		return err
	}
	_, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, msg)
	return err
}

func readLastApplyErr(db *sql.DB) string {
	if db == nil {
		return ""
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, "lanshare_last_err").Scan(&v); err != nil {
		return ""
	}
	return v
}

// EnrichStatus adds stored apply error to status.
func EnrichStatus(st Status, db *sql.DB) Status {
	if e := readLastApplyErr(db); e != "" {
		st.LastApplyErr = e
	}
	return st
}
