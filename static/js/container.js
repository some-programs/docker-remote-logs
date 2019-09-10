/* global conn_url, WebSocket, URL */

document.state = {
  autoscroll: document.getElementById("autoscroll").checked,
  wraplines: document.getElementById("wraplines").checked,
  timestamps: document.getElementById("timestamps").checked,
  ts_low: null,
  ts_high: null
};

function toggleSwitch() {
  const el = document.getElementById("switch");
  el.classList.toggle("hidden")
}

function switchLogs(id) {
  const u = new URL(window.location);
  u.searchParams.set("id", id);
  window.location.href = u.href;
}

function updateInterval() {
  var el = document.getElementById("interval");
  el.innerText = "" + document.state.ts_low + " - " + document.state.ts_high;
}

function buttonState(name) {
  var elm = document.getElementById(name);
  if (document.state[name]) {
    elm.classList.add("on");
    elm.classList.remove("off");
  } else {
    elm.classList.add("off");
    elm.classList.remove("on");
  }
}

buttonState("autoscroll");
buttonState("wraplines");
buttonState("timestamps");

function follow() {
  document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
  document.state.autoscroll = true;
  buttonState("autoscroll");
}

function toggleTimestamps() {
  document.state.timestamps = !document.state.timestamps;
  var elm = document.getElementById("log");
  if (document.state.timestamps) {
    elm.classList.remove("hidets");
  } else {
    elm.classList.add("hidets");
  }
  buttonState("timestamps");
}

function toggleWrap() {
  document.state.wraplines = !document.state.wraplines;
  if (document.state.wraplines) {
    document.getElementById("log").classList.add("wrap");
  } else {
    document.getElementById("log").classList.remove("wrap");
  }
  buttonState("wraplines");
}

function toggleScroll() {
  document.state.autoscroll = !document.state.autoscroll;
  buttonState("autoscroll");
}

var log = document.getElementById("log");
var conn = new WebSocket(conn_url);

conn.onopen = function(e) {
  window.setInterval(updateInterval, 450);
};
conn.onerror = function() {
  conn.close();
};
conn.onclose = function() {
  log.appendChild(document.createTextNode("\n\n-- CONNECTION TO LOG STREAM CLOSED --\n\n"));
};
conn.onmessage = function(e) {
  var ts = e.data.slice(0, 30);
  if (document.state.ts_low == null || ts < document.state.ts_low) {
    document.state.ts_low = ts;
  }
  if (document.state.ts_high == null || ts > document.state.ts_high) {
    document.state.ts_high = ts;
  }
  var tse = document.createElement("span");
  tse.textContent = ts;
  tse.setAttribute("class", "ts");
  log.appendChild(tse);

  log.appendChild(document.createTextNode(e.data.slice(31)));
  if (document.state.autoscroll && window.innerHeight + window.scrollY >= document.body.offsetHeight) {
    document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
  }
};
