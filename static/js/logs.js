"use strict";
/* global conn_url, WebSocket, URL */
class LogsApp {
    constructor() {
        this.ts_high = null;
        this.ts_low = null;
        this.connect = () => {
            var log = document.getElementById("log");
            var conn = new WebSocket(conn_url);
            conn.onopen = e => {
                setInterval(this.updateInterval, 450);
            };
            conn.onerror = () => {
                conn.close();
            };
            conn.onclose = () => {
                log.appendChild(document.createTextNode("\n\n-- CONNECTION TO LOG STREAM CLOSED --\n\n"));
            };
            conn.onmessage = e => {
                var ts = e.data.slice(0, 30);
                if (this.ts_low == null || ts < this.ts_low) {
                    this.ts_low = ts;
                }
                if (this.ts_high == null || ts > this.ts_high) {
                    this.ts_high = ts;
                }
                var tse = document.createElement("span");
                tse.textContent = ts;
                tse.setAttribute("class", "ts");
                log.appendChild(tse);
                log.appendChild(document.createTextNode(e.data.slice(31)));
                if (this.autoscroll && window.innerHeight + window.scrollY >= document.body.offsetHeight) {
                    document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
                }
            };
            return conn;
        };
        this.toggleSwitch = () => {
            const el = document.getElementById("switch");
            el.classList.toggle("hidden");
        };
        this.switchLogs = (id) => {
            const u = new URL(window.location.href);
            u.searchParams.set("id", id);
            window.location.href = u.href;
        };
        this.updateInterval = () => {
            var el = document.getElementById("interval");
            el.innerText = "" + this.ts_low + " - " + this.ts_high;
        };
        this.buttonState = (name) => {
            var elm = document.getElementById(name);
            if (this[name]) {
                elm.classList.add("on");
                elm.classList.remove("off");
            }
            else {
                elm.classList.add("off");
                elm.classList.remove("on");
            }
        };
        this.follow = () => {
            document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
            this.autoscroll = true;
            this.buttonState("autoscroll");
        };
        this.toggleTimestamps = () => {
            this.timestamps = !this.timestamps;
            var elm = document.getElementById("log");
            if (this.timestamps) {
                elm.classList.remove("hidets");
            }
            else {
                elm.classList.add("hidets");
            }
            this.buttonState("timestamps");
        };
        this.toggleWrap = () => {
            this.wraplines = !this.wraplines;
            if (this.wraplines) {
                document.getElementById("log").classList.add("wrap");
            }
            else {
                document.getElementById("log").classList.remove("wrap");
            }
            this.buttonState("wraplines");
        };
        this.toggleScroll = () => {
            this.autoscroll = !this.autoscroll;
            this.buttonState("autoscroll");
        };
        this.autoscroll = isChecked("autoscroll");
        this.wraplines = isChecked("wraplines");
        this.timestamps = isChecked("timestamps");
        this.buttonState("autoscroll");
        this.buttonState("wraplines");
        this.buttonState("timestamps");
        this.conn = this.connect();
    }
}
