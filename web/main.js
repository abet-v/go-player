(function () {
  const canvas = document.getElementById("board");
  const ctx = canvas.getContext("2d");
  const N = 19; // board size
  const margin = 20;
  const grid = (canvas.width - 2 * margin) / (N - 1);

  const roomId = location.pathname.split("/").pop();
  document.getElementById("roomId").textContent = `Room: ${roomId}`;
  document.getElementById("shareUrl").value =
    location.origin + location.pathname;

  let ws;
  let state = null;
  let myColor = "N";
  let hoverCell = null; // {i,j} when hovering a valid empty intersection during normal play

  function connect() {
    ws = new WebSocket(
      (location.protocol === "https:" ? "wss" : "ws") +
        "://" +
        location.host +
        "/ws/" +
        roomId
    );
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: "sync" }));
    };
    ws.onclose = () => {
      setStatus("Disconnected - retrying...");
      setTimeout(connect, 1000);
    };
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.type === "state") {
        state = msg;
        render();
        updateSidebar();
        setStatus("Connected" + (myColor !== "N" ? ` as ${myColor}` : ""));
      } else if (msg.type === "welcome") {
        myColor = msg.color || "N";
      } else if (msg.type === "error") {
        setStatus("Error: " + msg.error);
      }
    };
  }

  connect();

  function setStatus(s) {
    document.getElementById("status").textContent = s;
  }

  function toBoardCoord(x, y) {
    const i = Math.round((x - margin) / grid);
    const j = Math.round((y - margin) / grid);
    return { i, j };
  }
  // Hover preview handlers
  canvas.addEventListener("mousemove", (e) => {
    if (!state || state.over || state.scoring) {
      hoverCell = null;
      return;
    }
    const rect = canvas.getBoundingClientRect();
    const { i, j } = toBoardCoord(e.clientX - rect.left, e.clientY - rect.top);
    if (i < 0 || j < 0 || i >= N || j >= N) {
      hoverCell = null;
      render();
      return;
    }
    // Only allow hover on empty intersections
    if (state.board && state.board[i] && state.board[i][j] === "N") {
      hoverCell = { i, j };
    } else {
      hoverCell = null;
    }
    render();
  });
  canvas.addEventListener("mouseleave", () => {
    hoverCell = null;
    render();
  });

  canvas.addEventListener("click", (e) => {
    if (!state || state.over) return;
    if (state.scoring) {
      const rect = canvas.getBoundingClientRect();
      const { i, j } = toBoardCoord(
        e.clientX - rect.left,
        e.clientY - rect.top
      );
      if (i < 0 || j < 0 || i >= N || j >= N) return;
      ws.send(JSON.stringify({ type: "toggle-dead", x: i, y: j }));
      return;
    }
    const rect = canvas.getBoundingClientRect();
    const { i, j } = toBoardCoord(e.clientX - rect.left, e.clientY - rect.top);
    if (i < 0 || j < 0 || i >= N || j >= N) return;
    ws.send(JSON.stringify({ type: "place", x: i, y: j }));
  });

  document.getElementById("passBtn").onclick = () =>
    ws.send(JSON.stringify({ type: "pass" }));
  document.getElementById("resignBtn").onclick = () =>
    ws.send(JSON.stringify({ type: "resign" }));
  document.getElementById("scoreBtn").onclick = () =>
    ws.send(JSON.stringify({ type: "request-score" }));
  document.getElementById("finalizeBtn").onclick = () =>
    ws.send(JSON.stringify({ type: "finalize-score" }));

  function render() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    // draw grid
    ctx.strokeStyle = "#333";
    ctx.lineWidth = 1;
    for (let i = 0; i < N; i++) {
      const x = margin + i * grid;
      ctx.beginPath();
      ctx.moveTo(x, margin);
      ctx.lineTo(x, canvas.height - margin);
      ctx.stroke();
      const y = margin + i * grid;
      ctx.beginPath();
      ctx.moveTo(margin, y);
      ctx.lineTo(canvas.width - margin, y);
      ctx.stroke();
    }
    // star points for 19x19
    const stars = [3, 9, 15];
    ctx.fillStyle = "#333";
    stars.forEach((i) =>
      stars.forEach((j) => {
        dot(i, j);
      })
    );

    // stones
    if (!state) return;
    for (let i = 0; i < N; i++) {
      for (let j = 0; j < N; j++) {
        const s = state.board[i][j];
        if (s === "B" || s === "W") {
          stone(i, j, s);
        }
      }
    }

    // dead markers in scoring
    if (state.scoring && state.dead) {
      ctx.strokeStyle = "#f00";
      state.dead.forEach((p) => {
        drawX(p.X, p.Y);
      });
    }

    // hover ghost stone for current player's turn
    if (!state.over && !state.scoring && hoverCell) {
      const color = state.turn; // "B" or "W"
      ghostStone(hoverCell.i, hoverCell.j, color);
    }
  }

  function xy(i, j) {
    return { x: margin + i * grid, y: margin + j * grid };
  }
  function dot(i, j) {
    const p = xy(i, j);
    ctx.beginPath();
    ctx.arc(p.x, p.y, 3, 0, Math.PI * 2);
    ctx.fill();
  }

  function ghostStone(i, j, color) {
    const p = xy(i, j);
    const r = grid * 0.45;
    ctx.save();
    ctx.globalAlpha = 0.5; // semi-transparent
    // outline + fill with semi transparency, using solid color
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
    ctx.fillStyle = color === "B" ? "#000" : "#fff";
    ctx.fill();
    ctx.lineWidth = 2;
    ctx.strokeStyle = color === "B" ? "#222" : "#aaa";
    ctx.stroke();
    ctx.restore();
  }

  function stone(i, j, color) {
    const p = xy(i, j);
    const r = grid * 0.45;
    const grad = ctx.createRadialGradient(
      p.x - r * 0.3,
      p.y - r * 0.3,
      r * 0.1,
      p.x,
      p.y,
      r
    );
    if (color === "B") {
      grad.addColorStop(0, "#555");
      grad.addColorStop(1, "#000");
    } else {
      grad.addColorStop(0, "#fff");
      grad.addColorStop(1, "#ddd");
    }
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();
  }
  function drawX(i, j) {
    const p = xy(i, j);
    const r = grid * 0.3;
    ctx.beginPath();
    ctx.moveTo(p.x - r, p.y - r);
    ctx.lineTo(p.x + r, p.y + r);
    ctx.moveTo(p.x + r, p.y - r);
    ctx.lineTo(p.x - r, p.y + r);
    ctx.stroke();
  }

  function updateSidebar() {
    document.getElementById("turn").textContent = state.turn;
    document.getElementById("capB").textContent = state.captured.B;
    document.getElementById("capW").textContent = state.captured.W;
    document.getElementById("players").textContent =
      (state.players || []).map((p) => p.color).join(", ") || "—";
    if (state.over) {
      document.getElementById("result").textContent = "Result: " + state.result;
    } else if (state.scoring) {
      document.getElementById("result").textContent =
        "Scoring mode: click stones to mark dead, then Finalize";
    } else {
      document.getElementById("result").textContent = "";
    }
  }
})();
