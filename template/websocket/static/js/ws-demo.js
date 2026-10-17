// Connects the WebSocket demo card to GET /ws and reconnects when it drops.
(() => {
  const root = document.querySelector("[data-ws-demo]");
  if (!root) return;

  const log = root.querySelector("[data-ws-log]");
  const form = root.querySelector("form");
  const input = root.querySelector("input");
  const status = root.querySelector("[data-ws-status]");
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  let socket;

  const add = (text) => {
    const li = document.createElement("li");
    li.textContent = text;
    log.appendChild(li);
    log.scrollTop = log.scrollHeight;
  };

  const connect = () => {
    socket = new WebSocket(`${scheme}://${location.host}/ws`);
    socket.onopen = () => (status.textContent = "connected");
    socket.onmessage = (event) => add(event.data);
    socket.onclose = () => {
      status.textContent = "reconnecting…";
      setTimeout(connect, 1500);
    };
  };

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const text = input.value.trim();
    if (!text || socket.readyState !== WebSocket.OPEN) return;
    socket.send(text);
    input.value = "";
  });

  connect();
})();
