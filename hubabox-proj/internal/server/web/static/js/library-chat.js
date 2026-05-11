(function () {
	var form = document.getElementById("libChatForm");
	if (!form) return;

	var recBtn = document.getElementById("libVoiceRec");
	var stopBtn = document.getElementById("libVoiceStop");
	var fileInput = document.getElementById("libVoiceInput");
	var statusEl = document.getElementById("libVoiceStatus");
	var ta = form.querySelector('textarea[name="body"]');

	var mr = null;
	var chunks = [];
	var recordTimesliceMs = 400;

	function setStatus(t) {
		if (statusEl) statusEl.textContent = t || "";
	}

	function clearVoiceFile() {
		if (!fileInput) return;
		fileInput.value = "";
	}

	/** Browsers only expose microphone + MediaRecorder on secure contexts (https) or localhost. Plain http://LAN-IP blocks this in Chrome/Firefox. */
	function insecureForMic() {
		if (typeof window.isSecureContext === "boolean" && window.isSecureContext) {
			return false;
		}
		var h = (location.hostname || "").toLowerCase();
		return h !== "localhost" && h !== "127.0.0.1" && h !== "";
	}

	function pickRecorderMime() {
		if (!window.MediaRecorder || typeof MediaRecorder.isTypeSupported !== "function") {
			return "";
		}
		var mimes = [
			"audio/webm;codecs=opus",
			"audio/webm",
			"audio/ogg;codecs=opus",
			"audio/ogg",
			"video/webm;codecs=opus",
			"video/webm",
		];
		for (var i = 0; i < mimes.length; i++) {
			if (MediaRecorder.isTypeSupported(mimes[i])) {
				return mimes[i];
			}
		}
		return "";
	}

	function makeRecorder(stream) {
		var mime = pickRecorderMime();
		if (mime) {
			try {
				return new MediaRecorder(stream, { mimeType: mime });
			} catch (e) {
				/* fall through */
			}
		}
		return new MediaRecorder(stream);
	}

	function blobExtFromMime(mt) {
		mt = (mt || "").toLowerCase();
		if (mt.indexOf("ogg") !== -1) return "ogg";
		if (mt.indexOf("wav") !== -1) return "wav";
		return "webm";
	}

	if (insecureForMic()) {
		recBtn.disabled = true;
		stopBtn.disabled = true;
		setStatus(
			"Recording needs https:// or localhost — this page is plain HTTP on the LAN. Use “Attach audio file” or open the hub at 127.0.0.1 for recording."
		);
	} else if (!navigator.mediaDevices || typeof navigator.mediaDevices.getUserMedia !== "function") {
		recBtn.disabled = true;
		stopBtn.disabled = true;
		setStatus("This browser does not expose a microphone API here. Use file attach.");
	} else if (!window.MediaRecorder) {
		recBtn.disabled = true;
		stopBtn.disabled = true;
		setStatus("MediaRecorder not available. Use file attach.");
	}

	recBtn.addEventListener("click", function () {
		if (recBtn.disabled) return;
		if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
			setStatus("Recording not supported in this browser.");
			return;
		}
		clearVoiceFile();
		chunks = [];
		navigator.mediaDevices
			.getUserMedia({ audio: true })
			.then(function (stream) {
				mr = makeRecorder(stream);
				mr.ondataavailable = function (e) {
					if (e.data && e.data.size > 0) {
						chunks.push(e.data);
					}
				};
				mr.onerror = function () {
					setStatus("Recorder error — try file attach.");
				};
				mr.onstop = function () {
					stream.getTracks().forEach(function (t) {
						t.stop();
					});
					if (chunks.length === 0) {
						setStatus("No audio captured — try again or use file attach.");
						return;
					}
					var blob = new Blob(chunks, { type: mr.mimeType || "audio/webm" });
					var ext = blobExtFromMime(blob.type || mr.mimeType);
					var name = "voice-note." + ext;
					var file = new File([blob], name, { type: blob.type || "audio/webm" });
					try {
						var dt = new DataTransfer();
						dt.items.add(file);
						fileInput.files = dt.files;
						setStatus("Voice note attached — send when ready.");
					} catch (e) {
						setStatus("Could not attach recording — use file picker instead.");
					}
				};
				try {
					mr.start(recordTimesliceMs);
				} catch (e) {
					try {
						mr.start();
					} catch (e2) {
						stream.getTracks().forEach(function (t) {
							t.stop();
						});
						setStatus("Could not start recording — use file attach.");
						return;
					}
				}
				recBtn.disabled = true;
				stopBtn.disabled = false;
				setStatus("Recording…");
			})
			.catch(function (err) {
				var msg = "Microphone blocked or unavailable.";
				if (err && err.name === "NotAllowedError") {
					msg = "Microphone permission denied.";
				} else if (err && err.name === "NotFoundError") {
					msg = "No microphone found.";
				} else if (err && err.name === "NotSupportedError") {
					msg = "Recording not supported for this URL (try https or localhost).";
				}
				setStatus(msg + " Use file attach.");
			});
	});

	stopBtn.addEventListener("click", function () {
		if (!mr || mr.state === "inactive") {
			recBtn.disabled = false;
			stopBtn.disabled = true;
			mr = null;
			return;
		}
		if (typeof mr.requestData === "function") {
			try {
				mr.requestData();
			} catch (e) {
				/* ignore */
			}
		}
		mr.stop();
		mr = null;
		recBtn.disabled = false;
		stopBtn.disabled = true;
	});

	form.addEventListener("submit", function (e) {
		var body = ta && ta.value ? ta.value.trim() : "";
		var hasFile = fileInput && fileInput.files && fileInput.files.length > 0;
		if (!body && !hasFile) {
			e.preventDefault();
			setStatus("Type a message or attach a voice note.");
			return;
		}
		if (mr && mr.state === "recording") {
			e.preventDefault();
			setStatus("Stop recording before sending.");
			return;
		}
	});
})();
