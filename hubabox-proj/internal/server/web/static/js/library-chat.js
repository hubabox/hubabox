(function () {
	var form = document.getElementById("libChatForm");
	if (!form) return;

	var toggleBtn = document.getElementById("libVoiceToggle");
	var fileInput = document.getElementById("libVoiceInput");
	var statusEl = document.getElementById("libVoiceStatus");
	var sendBtn = document.getElementById("libChatSend");
	var ta = document.getElementById("libChatBody") || form.querySelector('textarea[name="body"]');

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

	function secureForMic() {
		if (typeof window.isSecureContext === "boolean" && window.isSecureContext) {
			return true;
		}
		var h = (location.hostname || "").toLowerCase();
		return h === "localhost" || h === "127.0.0.1" || h === "::1";
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

	function setRecordingUI(on) {
		if (!toggleBtn) return;
		toggleBtn.setAttribute("aria-pressed", on ? "true" : "false");
		toggleBtn.textContent = on ? "Stop" : "Record";
		toggleBtn.classList.toggle("lib-voice-toggle--active", on);
		syncSendEnabled();
	}

	function hasText() {
		return ta && ta.value && ta.value.trim().length > 0;
	}

	function hasClip() {
		return fileInput && fileInput.files && fileInput.files.length > 0;
	}

	function isRecording() {
		return mr && (mr.state === "recording" || mr.state === "paused");
	}

	function syncSendEnabled() {
		if (!sendBtn) return;
		if (isRecording()) {
			sendBtn.disabled = true;
			return;
		}
		sendBtn.disabled = !(hasText() || hasClip());
	}

	function micHardUnavailable(reason) {
		if (!toggleBtn) return;
		toggleBtn.disabled = true;
		setStatus(reason);
	}

	if (!toggleBtn || !sendBtn) return;

	if (!navigator.mediaDevices || typeof navigator.mediaDevices.getUserMedia !== "function") {
		micHardUnavailable("No microphone API — use Upload clip.");
	} else if (!window.MediaRecorder) {
		micHardUnavailable("Recording not supported — use Upload clip.");
	}

	syncSendEnabled();

	toggleBtn.addEventListener("click", function () {
		if (toggleBtn.disabled) return;

		if (mr && mr.state !== "inactive") {
			if (typeof mr.requestData === "function") {
				try {
					mr.requestData();
				} catch (e) {
					/* ignore */
				}
			}
			mr.stop();
			return;
		}

		if (!secureForMic()) {
			setStatus("Recording only works on https:// or localhost — use Upload clip on this address.");
			return;
		}

		if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
			setStatus("Recording not available.");
			return;
		}
		clearVoiceFile();
		syncSendEnabled();
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
					setStatus("Recorder error — try Upload clip.");
					try {
						stream.getTracks().forEach(function (t) {
							t.stop();
						});
					} catch (e2) {
						/* ignore */
					}
					mr = null;
					setRecordingUI(false);
				};
				mr.onstop = function () {
					stream.getTracks().forEach(function (t) {
						t.stop();
					});
					var savedMime = (mr && mr.mimeType) || "audio/webm";
					setRecordingUI(false);
					mr = null;
					if (chunks.length === 0) {
						setStatus("Nothing captured — try again or upload.");
						return;
					}
					var blob = new Blob(chunks, { type: savedMime });
					var ext = blobExtFromMime(blob.type || savedMime);
					var name = "voice-note." + ext;
					var file = new File([blob], name, { type: blob.type || savedMime });
					try {
						var dt = new DataTransfer();
						dt.items.add(file);
						fileInput.files = dt.files;
						setStatus("Clip attached — press Send.");
					} catch (e) {
						setStatus("Could not attach — use Upload clip.");
					}
					syncSendEnabled();
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
						mr = null;
						setRecordingUI(false);
						setStatus("Could not start — use Upload clip.");
						return;
					}
				}
				setRecordingUI(true);
				setStatus("Recording…");
			})
			.catch(function (err) {
				var msg = "Mic unavailable.";
				if (err && err.name === "NotAllowedError") msg = "Mic blocked.";
				else if (err && err.name === "NotFoundError") msg = "No mic found.";
				setStatus(msg + " Use Upload clip.");
			});
	});

	if (fileInput) {
		fileInput.addEventListener("change", function () {
			if (fileInput.files && fileInput.files.length > 0) {
				setStatus("Clip attached — press Send.");
			} else {
				setStatus("");
			}
			syncSendEnabled();
		});
	}

	if (ta) {
		ta.addEventListener("input", syncSendEnabled);
	}

	form.addEventListener("submit", function (e) {
		var body = ta && ta.value ? ta.value.trim() : "";
		var hasFile = fileInput && fileInput.files && fileInput.files.length > 0;
		if (!body && !hasFile) {
			e.preventDefault();
			setStatus("Add text or a clip.");
			return;
		}
		if (isRecording()) {
			e.preventDefault();
			setStatus("Stop recording first.");
			return;
		}
	});
})();

(function () {
	var host = document.getElementById("lib-chat-msgs-host");
	if (!host) return;

	var savedScroll = 0;
	var stickBottom = false;
	var bottomSlackPx = 80;

	function anyChatAudioPlaying() {
		var list = host.querySelectorAll("audio");
		for (var i = 0; i < list.length; i++) {
			var a = list[i];
			if (!a.paused && !a.ended) return true;
		}
		return false;
	}

	host.addEventListener("htmx:beforeRequest", function (e) {
		if (e.detail && e.detail.elt !== host) return;
		if (anyChatAudioPlaying()) {
			e.preventDefault();
		}
	});

	host.addEventListener("htmx:beforeSwap", function (e) {
		if (!e.detail || e.detail.target !== host) return;
		stickBottom = host.scrollHeight - host.scrollTop - host.clientHeight < bottomSlackPx;
		savedScroll = host.scrollTop;
	});

	host.addEventListener("htmx:afterSwap", function (e) {
		if (!e.detail || e.detail.target !== host) return;
		var maxScroll = Math.max(0, host.scrollHeight - host.clientHeight);
		if (stickBottom) {
			host.scrollTop = maxScroll;
		} else {
			host.scrollTop = Math.min(savedScroll, maxScroll);
		}
	});
})();
