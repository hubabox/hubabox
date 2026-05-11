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

	function postFiles(url, fileList, onDone) {
		var i = 0;
		function next() {
			if (i >= fileList.length) {
				onDone(null);
				return;
			}
			var fd = new FormData();
			fd.append("file", fileList[i]);
			i++;
			fetch(url, { method: "POST", body: fd, credentials: "same-origin" })
				.then(function (res) {
					if (!res.ok) throw new Error("HTTP " + res.status);
					next();
				})
				.catch(function (e) {
					onDone(e);
				});
		}
		next();
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
				postFiles(url, files, function (err) {
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
