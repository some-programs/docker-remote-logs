"use strict";
class EventsApp {
    constructor() {
        this.connect = () => {
            var log = document.getElementById("log");
            var conn = new WebSocket(conn_url);
            conn.onerror = () => {
                conn.close();
            };
            conn.onclose = () => {
                log.appendChild(document.createTextNode("\n\n-- CONNECTION TO EVENTS STREAM CLOSED --\n\n"));
            };
            conn.onmessage = e => {
                log.appendChild(document.createTextNode(e.data));
            };
            return conn;
        };
        this.conn = this.connect();
    }
}
