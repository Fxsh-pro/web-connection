const state = {
    ws: null,
    clientId: null,
    roomId: null,
    peers: new Map(), // peerId -> { pc, dc, stream, videoEl }
    localStream: null,
    constraints: {
        audio: {
            noiseSuppression: true,
            echoCancellation: true,
            autoGainControl: true
        },
        video: {
            width: {ideal: 1280},
            height: {ideal: 720}
        }
    }
};

const selfVideo = document.getElementById('selfVideo');
const selfLabel = document.getElementById('selfLabel');
const roomInput = document.getElementById('roomId');
const displayNameInput = document.getElementById('displayName');
const joinBtn = document.getElementById('joinBtn');
const leaveBtn = document.getElementById('leaveBtn');
const toggleAudioBtn = document.getElementById('toggleAudio');
const toggleVideoBtn = document.getElementById('toggleVideo');
const videos = document.getElementById('videos');
const chatHistory = document.getElementById('chatHistory');
const chatForm = document.getElementById('chatForm');
const chatInput = document.getElementById('chatInput');
const chatSend = document.getElementById('chatSend');
const noiseSuppression = document.getElementById('noiseSuppression');
const echoCancellation = document.getElementById('echoCancellation');
const autoGainControl = document.getElementById('autoGainControl');

function log(...args) {
    console.log('[web]', ...args);
}

async function ensureLocalStream() {
    if (state.localStream) return state.localStream;
    state.constraints.audio.noiseSuppression = !!noiseSuppression.checked;
    state.constraints.audio.echoCancellation = !!echoCancellation.checked;
    state.constraints.audio.autoGainControl = !!autoGainControl.checked;
    state.localStream = await navigator.mediaDevices.getUserMedia(state.constraints);
    selfVideo.srcObject = state.localStream;
    enableControls(true);
    return state.localStream;
}

function enableControls(enabled) {
    toggleAudioBtn.disabled = !enabled;
    toggleVideoBtn.disabled = !enabled;
}

function setChatEnabled(requestedEnabled) {
    const joined = !!state.roomId && !!state.ws && state.ws.readyState === WebSocket.OPEN;
    const enabled = !!requestedEnabled && joined;
    if (chatInput) chatInput.disabled = !enabled;
    if (chatSend) chatSend.disabled = !enabled;
}

function connectWS(roomId) {
    const name = (displayNameInput?.value || '').trim();
    const qsName = name ? `&name=${encodeURIComponent(name)}` : '';
    const wsUrl = (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + `/ws?room=${encodeURIComponent(roomId)}${qsName}`;
    const ws = new WebSocket(wsUrl);
    state.ws = ws;
    ws.onopen = () => {
        log('ws open');
        setChatEnabled(true);
    };
    ws.onclose = () => {
        log('ws closed');
        setChatEnabled(false);
    };
    ws.onerror = (e) => log('ws error', e);
    ws.onmessage = async (ev) => {
        const msg = JSON.parse(ev.data);
        switch (msg.type) {
            case 'welcome':
                state.clientId = msg.from;
                break;
            case 'peers':
                // Set own label
                if (selfLabel) selfLabel.textContent = name || 'You';
                for (const peer of msg.peers || []) {
                    await createPeerConnection(peer.id, true, peer.name);
                }
                break;
            case 'peer-joined':
                await createPeerConnection(msg.from, true);
                break;
            case 'peer-left':
                closePeer(msg.from);
                break;
            case 'signal':
                await handleSignal(msg);
                break;
            case 'chat-history':
                (msg.messages || []).forEach(appendChat);
                break;
            case 'chat':
                appendChat(msg.msg);
                break;
        }
    };
}

async function handleSignal({
    from,
    data
}) {
    const payload = JSON.parse(data);
    let peer = state.peers.get(from);
    if (!peer) {
        peer = await createPeerConnection(from, false);
    }
    if (payload.sdp) {
        await peer.pc.setRemoteDescription(payload.sdp);
        if (payload.sdp.type === 'offer') {
            const answer = await peer.pc.createAnswer();
            await peer.pc.setLocalDescription(answer);
            sendSignal(from, {sdp: peer.pc.localDescription});
        }
    } else if (payload.candidate) {
        try {
            await peer.pc.addIceCandidate(payload.candidate);
        } catch (_) {
        }
    }
}

function sendSignal(to, payload) {
    state.ws?.send(JSON.stringify({
        type: 'signal',
        to,
        data: JSON.stringify(payload)
    }));
}

function createVideoTile(labelText) {
    const wrap = document.createElement('div');
    wrap.className = 'video-tile';
    const vid = document.createElement('video');
    vid.autoplay = true;
    vid.playsInline = true;
    const label = document.createElement('div');
    label.className = 'label';
    label.textContent = labelText;
    wrap.appendChild(vid);
    wrap.appendChild(label);
    videos.appendChild(wrap);
    return {
        wrap,
        vid
    };
}

async function createPeerConnection(peerId, isInitiator, peerName) {
    if (state.peers.has(peerId)) return state.peers.get(peerId);
    await ensureLocalStream();
    const pc = new RTCPeerConnection({
        iceServers: [{urls: 'stun:stun.l.google.com:19302'}]
    });
    state.localStream.getTracks().forEach(t => pc.addTrack(t, state.localStream));

    const {vid} = createVideoTile(peerName || peerId);
    const remoteStream = new MediaStream();
    vid.srcObject = remoteStream;
    pc.addEventListener('track', ev => {
        remoteStream.addTrack(ev.track);
    });
    pc.addEventListener('icecandidate', ev => {
        if (ev.candidate) sendSignal(peerId, {candidate: ev.candidate});
    });
    pc.addEventListener('connectionstatechange', () => {
        if (['failed', 'disconnected', 'closed'].includes(pc.connectionState)) {
            closePeer(peerId);
        }
    });

    let dc = null;
    try {
        dc = pc.createDataChannel('chat');
    } catch (_) {
    }

    const peerObj = {
        pc,
        dc,
        stream: remoteStream,
        videoEl: vid
    };
    state.peers.set(peerId, peerObj);

    if (isInitiator) {
        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        sendSignal(peerId, {sdp: pc.localDescription});
    }
    return peerObj;
}

function closePeer(peerId) {
    const peer = state.peers.get(peerId);
    if (!peer) return;
    try {
        peer.pc.close();
    } catch (_) {
    }
    state.peers.delete(peerId);
    // remove video element
    const tile = peer.videoEl?.parentElement;
    if (tile && tile.parentElement) tile.parentElement.removeChild(tile);
}

async function joinRoom() {
    const rid = roomInput.value.trim();
    if (!rid) return alert('Enter room ID');
    await ensureLocalStream();
    connectWS(rid);
    state.roomId = rid;
    joinBtn.disabled = true;
    leaveBtn.disabled = false;
    roomInput.disabled = true;
    setChatEnabled(false); // enable on ws open
}

function leaveRoom() {
    state.ws?.close();
    state.ws = null;
    for (const pid of Array.from(state.peers.keys())) closePeer(pid);
    joinBtn.disabled = false;
    leaveBtn.disabled = true;
    roomInput.disabled = false;
    setChatEnabled(false);
}

function toggleAudio() {
    if (!state.localStream) return;
    const track = state.localStream.getAudioTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    toggleAudioBtn.textContent = track.enabled ? 'Mute' : 'Unmute';
    toggleAudioBtn.classList.toggle('on', track.enabled);
}

function toggleVideo() {
    if (!state.localStream) return;
    const track = state.localStream.getVideoTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    toggleVideoBtn.textContent = track.enabled ? 'Video Off' : 'Video On';
    toggleVideoBtn.classList.toggle('on', track.enabled);
}

joinBtn.addEventListener('click', joinRoom);
leaveBtn.addEventListener('click', leaveRoom);
toggleAudioBtn.addEventListener('click', toggleAudio);
toggleVideoBtn.addEventListener('click', toggleVideo);

window.addEventListener('beforeunload', () => {
    state.ws?.close();
});

function appendChat(cm) {
    const row = document.createElement('div');
    row.className = 'chat-msg';
    const meta = document.createElement('div');
    meta.className = 'meta';
    const who = cm.senderName || cm.sender?.slice(0, 6) || 'peer';
    meta.textContent = `${who} · ${new Date(cm.createdAt || Date.now()).toLocaleTimeString()}`;
    const body = document.createElement('div');
    body.className = 'body';
    body.textContent = cm.body;
    row.appendChild(meta);
    row.appendChild(body);
    chatHistory.appendChild(row);
    chatHistory.scrollTop = chatHistory.scrollHeight;
}

chatForm?.addEventListener('submit', (e) => {
    e.preventDefault();
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN || !state.roomId) return;
    const text = (chatInput.value || '').trim();
    if (!text) return;
    // Send as simple string to avoid nested JSON quoting
    state.ws?.send(JSON.stringify({
        type: 'chat',
        data: JSON.stringify(text)
    }));
    chatInput.value = '';
});


