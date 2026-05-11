(function () {
	function initFileFilters() {
		document.querySelectorAll(".file-filter").forEach(function (inp) {
			inp.addEventListener("input", function () {
				var q = (inp.value || "").toLowerCase().trim();
				var root = inp.closest(".file-browser");
				if (!root) return;
				root.querySelectorAll(".filelist li").forEach(function (li) {
					var name = (li.getAttribute("data-name") || "").toLowerCase();
					li.hidden = q.length > 0 && name.indexOf(q) === -1;
				});
			});
		});
	}

	function uploadURL(el) {
		return el.getAttribute("data-upload-url") || "";
	}

	/** One multipart/form-data POST with all parts named "files" (same as multi-select form). */
	function postFilesBatch(url, fileList, onDone) {
		var fd = new FormData();
		for (var i = 0; i < fileList.length; i++) {
			fd.append("files", fileList[i]);
		}
		fetch(url, { method: "POST", body: fd, credentials: "same-origin" })
			.then(function (res) {
				if (!res.ok) throw new Error("HTTP " + res.status);
				onDone(null);
			})
			.catch(function (e) {
				onDone(e);
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
				postFilesBatch(url, files, function (err) {
					el.classList.remove("dropzone--busy");
					if (err) {
						window.alert("Upload failed. Check file size and try again.");
						return;
					}
					window.location.reload();
				});
			});
		});
	}

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", function () {
			initFileFilters();
			initDropzones();
		});
	} else {
		initFileFilters();
		initDropzones();
	}
})();
