package server

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kros/hubabox/internal/lanshare"
	"github.com/kros/hubabox/internal/netutil"
)

func (s *Server) filesLANShareEnablePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=lanshare_err&why="+url.QueryEscape("bad form"), http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.FormValue("share_name"))
	if name != "" {
		if err := lanshare.SetShareName(s.db, name); err != nil {
			http.Redirect(w, r, "/files?msg=lanshare_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
	}
	host, _ := os.Hostname()
	lanIPs, _ := netutil.LANIPv4Strings()
	st := lanshare.Apply(s.db, s.cfg.DataDir, s.filesDir, host, lanIPs)
	if st.Active {
		http.Redirect(w, r, "/files?msg=lanshare_on", http.StatusSeeOther)
		return
	}
	why := st.LastApplyErr
	if why == "" {
		why = "share not active — run the helper script as Administrator or root"
	}
	http.Redirect(w, r, "/files?msg=lanshare_pending&why="+url.QueryEscape(why), http.StatusSeeOther)
}

func (s *Server) filesLANShareDisablePost(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	lanIPs, _ := netutil.LANIPv4Strings()
	_ = lanshare.Disable(s.db, s.cfg.DataDir, s.filesDir, host, lanIPs)
	http.Redirect(w, r, "/files?msg=lanshare_off", http.StatusSeeOther)
}
