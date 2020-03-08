"use strict";
/* global conn_url, WebSocket, URL */
const state = {
    autoscroll: document.getElementById("autoscroll").checked,
    wraplines: document.getElementById("wraplines").checked,
    timestamps: document.getElementById("timestamps").checked,
    ts_low: null,
    ts_high: null
};
window.state = state;
function toggleSwitch() {
    const el = document.getElementById("switch");
    el.classList.toggle("hidden");
}
function switchLogs(id) {
    const u = new URL(window.location.href);
    u.searchParams.set("id", id);
    window.location.href = u.href;
}
function updateInterval() {
    var el = document.getElementById("interval");
    el.innerText = "" + window.state.ts_low + " - " + window.state.ts_high;
}
function buttonState(name) {
    var elm = document.getElementById(name);
    if (window.state[name]) {
        elm.classList.add("on");
        elm.classList.remove("off");
    }
    else {
        elm.classList.add("off");
        elm.classList.remove("on");
    }
}
buttonState("autoscroll");
buttonState("wraplines");
buttonState("timestamps");
function follow() {
    document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
    window.state.autoscroll = true;
    buttonState("autoscroll");
}
function toggleTimestamps() {
    window.state.timestamps = !window.state.timestamps;
    var elm = document.getElementById("log");
    if (window.state.timestamps) {
        elm.classList.remove("hidets");
    }
    else {
        elm.classList.add("hidets");
    }
    buttonState("timestamps");
}
function toggleWrap() {
    window.state.wraplines = !window.state.wraplines;
    if (window.state.wraplines) {
        document.getElementById("log").classList.add("wrap");
    }
    else {
        document.getElementById("log").classList.remove("wrap");
    }
    buttonState("wraplines");
}
function toggleScroll() {
    window.state.autoscroll = !window.state.autoscroll;
    buttonState("autoscroll");
}
var log = document.getElementById("log");
var conn = new WebSocket(conn_url);
conn.onopen = function (e) {
    window.setInterval(updateInterval, 450);
};
conn.onerror = function () {
    conn.close();
};
conn.onclose = function () {
    log.appendChild(document.createTextNode("\n\n-- CONNECTION TO LOG STREAM CLOSED --\n\n"));
};
conn.onmessage = function (e) {
    var ts = e.data.slice(0, 30);
    if (window.state.ts_low == null || ts < window.state.ts_low) {
        window.state.ts_low = ts;
    }
    if (window.state.ts_high == null || ts > window.state.ts_high) {
        window.state.ts_high = ts;
    }
    var tse = document.createElement("span");
    tse.textContent = ts;
    tse.setAttribute("class", "ts");
    log.appendChild(tse);
    log.appendChild(document.createTextNode(e.data.slice(31)));
    if (window.state.autoscroll && window.innerHeight + window.scrollY >= document.body.offsetHeight) {
        document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
    }
};
