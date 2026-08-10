(function () {
	function scrollSnapshotPrefix() {
		var path = window.location.pathname || "";
		if (path === "/files" || path.indexOf("/files/") === 0) {
			return "files";
		}
		if (path === "/library" || path.indexOf("/library/") === 0) {
			return "library";
		}
		return "";
	}

	function saveScrollSnapshot() {
		var prefix = scrollSnapshotPrefix();
		if (!prefix) {
			return;
		}
		try {
			sessionStorage.setItem("hubabox_ss_" + prefix + "_y", String(window.scrollY));
			var fb = document.querySelector(".file-browser-scroll");
			if (fb) {
				sessionStorage.setItem("hubabox_ss_" + prefix + "_fb", String(fb.scrollTop));
			}
			var ch = document.querySelector(".lib-chat-scroll-host");
			if (ch) {
				sessionStorage.setItem("hubabox_ss_" + prefix + "_ch", String(ch.scrollTop));
			}
		} catch (e) {
			/* ignore */
		}
	}

	function restoreScrollSnapshot() {
		var prefix = scrollSnapshotPrefix();
		if (!prefix) {
			return;
		}
		var y = null;
		var fbv = null;
		var chv = null;
		try {
			y = sessionStorage.getItem("hubabox_ss_" + prefix + "_y");
			fbv = sessionStorage.getItem("hubabox_ss_" + prefix + "_fb");
			chv = sessionStorage.getItem("hubabox_ss_" + prefix + "_ch");
			sessionStorage.removeItem("hubabox_ss_" + prefix + "_y");
			sessionStorage.removeItem("hubabox_ss_" + prefix + "_fb");
			sessionStorage.removeItem("hubabox_ss_" + prefix + "_ch");
		} catch (e) {
			return;
		}
		requestAnimationFrame(function () {
			if (y !== null) {
				window.scrollTo(0, parseInt(y, 10) || 0);
			}
			requestAnimationFrame(function () {
				var fb = document.querySelector(".file-browser-scroll");
				if (fb && fbv !== null) {
					fb.scrollTop = parseInt(fbv, 10) || 0;
				}
				var ch = document.querySelector(".lib-chat-scroll-host");
				if (ch && chv !== null) {
					ch.scrollTop = parseInt(chv, 10) || 0;
				}
			});
		});
	}

	function initScrollRestoreOnSubmit() {
		document.addEventListener(
			"submit",
			function (e) {
				var t = e.target;
				if (!(t instanceof HTMLFormElement)) {
					return;
				}
				var action = (t.getAttribute("action") || "").trim();
				if (!action) {
					return;
				}
				try {
					var u = new URL(action, window.location.href);
					if (u.origin !== window.location.origin) {
						return;
					}
					var p = u.pathname;
					if (p.indexOf("/files") === 0 || p.indexOf("/library") === 0) {
						saveScrollSnapshot();
					}
				} catch (err) {
					/* ignore */
				}
			},
			true
		);
	}

	function kindMatches(liKind, filterVal) {
		if (!filterVal) {
			return true;
		}
		if (filterVal === "documents") {
			return liKind === "doc" || liKind === "sheet" || liKind === "slides";
		}
		return liKind === filterVal;
	}

	function nameMatches(nameLower, q) {
		if (!q.length) {
			return true;
		}
		return nameLower.indexOf(q) !== -1;
	}

	function updateFileFilterSummary(root, visible, total) {
		var el = root.querySelector(".file-filter-summary");
		if (!el) {
			return;
		}
		if (total === 0) {
			el.hidden = true;
			el.textContent = "";
			return;
		}
		el.hidden = false;
		if (visible === total) {
			el.textContent = "Showing all " + total + " file" + (total === 1 ? "" : "s") + ".";
		} else if (visible === 0) {
			el.textContent = "No files match these filters.";
		} else {
			el.textContent = "Showing " + visible + " of " + total + " file" + (total === 1 ? "" : "s") + ".";
		}
	}

	function applyFileBrowserFilters(root) {
		var inp = root.querySelector(".file-filter");
		var sel = root.querySelector(".file-kind-filter");
		var q = inp ? (inp.value || "").toLowerCase().trim() : "";
		var kindVal = sel ? (sel.value || "").trim() : "";
		var items = root.querySelectorAll(".filelist li");
		var total = 0;
		var visible = 0;
		items.forEach(function (li) {
			total++;
			var name = (li.getAttribute("data-name") || "").toLowerCase();
			var liKind = (li.getAttribute("data-kind") || "").toLowerCase();
			var show = nameMatches(name, q) && kindMatches(liKind, kindVal);
			li.hidden = !show;
			if (show) {
				visible++;
			}
		});
		updateFileFilterSummary(root, visible, total);
	}

	function initFileFilters() {
		document.querySelectorAll(".file-browser").forEach(function (root) {
			var inp = root.querySelector(".file-filter");
			var sel = root.querySelector(".file-kind-filter");
			if (!inp && !sel) {
				return;
			}
			function run() {
				applyFileBrowserFilters(root);
			}
			if (inp) {
				inp.addEventListener("input", run);
				inp.addEventListener("change", run);
				/* type=search: clear (x) and Enter often fire `search`, not `input`, on some engines */
				inp.addEventListener("search", run);
			}
			if (sel) {
				sel.addEventListener("change", run);
			}
			run();
		});
	}

	function initFileInsightModal() {
		var modal = document.getElementById("hubFileInsightModal");
		if (!modal) {
			return;
		}
		var bodyEl = modal.querySelector(".file-insight-modal__body");
		if (!bodyEl) {
			return;
		}

		function onDocKey(e) {
			if (e.key === "Escape" && !modal.hidden) {
				closeModal();
			}
		}

		function closeModal() {
			modal.hidden = true;
			modal.setAttribute("aria-hidden", "true");
			bodyEl.innerHTML = "";
			document.body.style.overflow = "";
			document.removeEventListener("keydown", onDocKey);
		}

		function openModal() {
			modal.hidden = false;
			modal.setAttribute("aria-hidden", "false");
			document.body.style.overflow = "hidden";
			document.addEventListener("keydown", onDocKey);
		}

		modal.addEventListener("click", function (e) {
			if (e.target.closest("[data-close-insight-modal]")) {
				closeModal();
			}
		});

		document.addEventListener("click", function (e) {
			var btn = e.target.closest("[data-insight-url]");
			if (!btn || !(btn instanceof HTMLElement)) {
				return;
			}
			var url = btn.getAttribute("data-insight-url");
			if (!url) {
				return;
			}
			e.preventDefault();
			bodyEl.innerHTML = '<p class="muted">Loading…</p>';
			openModal();
			fetch(url, { credentials: "same-origin" })
				.then(function (res) {
					if (!res.ok) {
						throw new Error(String(res.status));
					}
					return res.text();
				})
				.then(function (html) {
					bodyEl.innerHTML = html;
				})
				.catch(function () {
					bodyEl.innerHTML = '<p class="err">Could not load file details.</p>';
				});
		});
	}

	function initFileBulkDelete() {
		var form = document.getElementById("hubFilesBulkDeleteForm");
		if (!form) {
			return;
		}
		var selVis = document.getElementById("hubFilesSelectVisible");
		var clr = document.getElementById("hubFilesClearSelection");
		if (selVis) {
			selVis.addEventListener("click", function () {
				form.querySelectorAll(".filelist li:not([hidden]) .file-delete-cb").forEach(function (c) {
					c.checked = true;
				});
			});
		}
		if (clr) {
			clr.addEventListener("click", function () {
				form.querySelectorAll(".file-delete-cb").forEach(function (c) {
					c.checked = false;
				});
			});
		}
		form.addEventListener("submit", function (e) {
			var boxes = form.querySelectorAll(".file-delete-cb:checked");
			if (!boxes.length) {
				e.preventDefault();
				window.alert("Select at least one file.");
				return;
			}
			if (
				!window.confirm(
					"Delete " +
						boxes.length +
						" selected file" +
						(boxes.length === 1 ? "" : "s") +
						"? This cannot be undone."
				)
			) {
				e.preventDefault();
			}
		});
	}

	function uploadURL(el) {
		return el.getAttribute("data-upload-url") || "";
	}

	function appendFiles(fd, fileList) {
		for (var i = 0; i < fileList.length; i++) {
			var f = fileList[i];
			var rel =
				f.webkitRelativePath && f.webkitRelativePath.length > 0
					? f.webkitRelativePath.replace(/\\/g, "/")
					: f.name;
			fd.append("files", f, rel);
		}
	}

	function uploadProgress(root, loaded, total) {
		if (!root) return;
		var status = root.querySelector("[data-upload-status]");
		var meter = root.querySelector("[data-upload-meter]");
		var percent = root.querySelector("[data-upload-percent]");
		root.hidden = false;
		if (total > 0) {
			var p = Math.min(100, Math.round((loaded / total) * 100));
			if (meter) meter.value = p;
			if (p >= 100) {
				root.classList.add("upload-progress--saving");
				if (percent) percent.textContent = "100%";
				if (status) status.textContent = "Upload received — saving it to the hub…";
			} else {
				root.classList.remove("upload-progress--saving");
				if (percent) percent.textContent = p + "%";
				if (status) status.textContent = "Uploading…";
			}
		} else if (status) {
			status.textContent = "Uploading…";
		}
	}

	function uploadFinished(root) {
		if (!root) return;
		root.classList.remove("upload-progress--saving");
		var status = root.querySelector("[data-upload-status]");
		if (status) status.textContent = "Saving file…";
	}

	function postFormData(url, fd, root, onDone) {
		var req = new XMLHttpRequest();
		req.open("POST", url, true);
		req.withCredentials = true;
		req.upload.addEventListener("progress", function (e) {
			uploadProgress(root, e.loaded, e.lengthComputable ? e.total : 0);
		});
		req.addEventListener("load", function () {
			if (req.status >= 200 && req.status < 400) {
				onDone(null, req.responseURL);
				return;
			}
			onDone(new Error("HTTP " + req.status));
		});
		req.addEventListener("error", function () { onDone(new Error("network error")); });
		req.addEventListener("abort", function () { onDone(new Error("upload cancelled")); });
		req.send(fd);
	}

	/** One multipart/form-data POST with all parts named "files" (same as multi-select form). */
	function postFilesBatch(url, fileList, root, onDone) {
		var fd = new FormData();
		var csrf = document.querySelector('meta[name="hubabox-csrf"]');
		if (csrf) fd.append("csrf_token", csrf.content);
		appendFiles(fd, fileList);
		postFormData(url, fd, root, onDone);
	}

	function initHelpModal() {
		var modal = document.getElementById("hubHelpModal");
		if (!modal) {
			return;
		}
		var bodyEl = modal.querySelector(".hub-help-modal__body");
		var titleEl = document.getElementById("hubHelpModalTitle");
		if (!bodyEl) {
			return;
		}

		function closeModal() {
			modal.hidden = true;
			modal.setAttribute("aria-hidden", "true");
			bodyEl.innerHTML = "";
		}

		function openHelp(helpId) {
			var src = document.getElementById("help-content-" + helpId);
			if (!src) {
				return;
			}
			bodyEl.innerHTML = src.innerHTML;
			if (titleEl) {
				var h = src.querySelector(".hub-help-heading");
				titleEl.textContent = h ? h.textContent : "Help";
			}
			modal.hidden = false;
			modal.setAttribute("aria-hidden", "false");
			var closeBtn = modal.querySelector("[data-close-help-modal]");
			if (closeBtn && closeBtn.focus) {
				closeBtn.focus();
			}
		}

		document.addEventListener("keydown", function (e) {
			if (e.key === "Escape" && !modal.hidden) {
				closeModal();
			}
		});

		modal.addEventListener("click", function (e) {
			if (e.target.closest("[data-close-help-modal]")) {
				closeModal();
			}
		});

		document.querySelectorAll(".help-icon[data-help-id]").forEach(function (btn) {
			btn.addEventListener("click", function (e) {
				e.preventDefault();
				e.stopPropagation();
				openHelp(btn.getAttribute("data-help-id"));
			});
		});
	}

	function initDropzones() {
		document.querySelectorAll(".dropzone").forEach(function (el) {
			var url = uploadURL(el);
			if (!url) return;

			["dragenter", "dragover", "dragleave", "drop"].forEach(function (ev) {
				el.addEventListener(ev, function (e) {
					e.preventDefault();
					e.stopPropagation();
				});
			});
			el.addEventListener("dragenter", function () {
				el.classList.add("dropzone--active");
			});
			el.addEventListener("dragleave", function () {
				el.classList.remove("dropzone--active");
			});
			el.addEventListener("drop", function (e) {
				el.classList.remove("dropzone--active");
				var files = e.dataTransfer && e.dataTransfer.files;
				if (!files || !files.length) return;
				el.classList.add("dropzone--busy");
				var progress = el.parentElement.querySelector("[data-upload-progress]");
				postFilesBatch(url, files, progress, function (err, resultURL) {
					el.classList.remove("dropzone--busy");
					if (err) {
						window.alert("Upload failed: " + err.message);
						return;
					}
					uploadFinished(progress);
					saveScrollSnapshot();
					window.location.assign(resultURL || "/files");
				});
			});
		});
	}

	function initUploadForms() {
		document.querySelectorAll("form[data-upload-form]").forEach(function (form) {
			form.addEventListener("submit", function (e) {
				e.preventDefault();
				var inputs = form.querySelectorAll('input[type="file"][name="files"]');
				var files = [];
				inputs.forEach(function (input) {
					for (var i = 0; i < input.files.length; i++) files.push(input.files[i]);
				});
				if (!files.length) {
					window.alert("Choose at least one file.");
					return;
				}
				var button = form.querySelector('button[type="submit"]');
				var progress = form.parentElement.querySelector("[data-upload-progress]");
				if (button) button.disabled = true;
				postFilesBatch(form.action, files, progress, function (err, resultURL) {
					if (button) button.disabled = false;
					if (err) {
						window.alert("Upload failed: " + err.message);
						return;
					}
					uploadFinished(progress);
					saveScrollSnapshot();
					window.location.assign(resultURL || "/files");
				});
			});
		});
	}

	function boot() {
		initThemeToggle();
		initScrollRestoreOnSubmit();
		initFileFilters();
		initFileInsightModal();
		initHelpModal();
		initFileBulkDelete();
		initDropzones();
		initUploadForms();
		restoreScrollSnapshot();
	}

	function initThemeToggle() {
		var button = document.querySelector("[data-theme-toggle]");
		if (!button) return;
		function label() { button.textContent = document.documentElement.dataset.theme === "light" ? "Dark mode" : "Light mode"; }
		button.addEventListener("click", function () {
			var next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
			document.documentElement.dataset.theme = next;
			try { localStorage.setItem("hubabox-theme", next); } catch (e) { /* ignore */ }
			label();
		});
		label();
	}

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", boot);
	} else {
		boot();
	}
})();
