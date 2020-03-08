"use strict";
/* global localStorage */
class IndexApp {
    constructor() {
        this.updateFields = (e) => {
            var el = e.target;
            if (hasText("tail")) {
                localStorage.setItem("tail", getText("tail"));
            }
            if (el.id == "since" && hasText("since")) {
                empty("tail");
                uncheck("follow");
            }
            if (el.id == "until" && hasText("until")) {
                empty("tail");
                uncheck("follow");
            }
            if (el.id == "tail" && hasText("tail")) {
                empty("since");
                empty("until");
            }
            if (el.id == "follow" && isChecked("follow")) {
                empty("since");
                empty("until");
            }
        };
        this.checkbox = (name) => {
            if (getInputElementById(name).checked) {
                return "&" + name + "=true";
            }
            return "&" + name + "=false";
        };
        this.text = (name) => {
            if (getInputElementById(name).value === "") {
                return "";
            }
            return `&${name}=${getInputElementById(name).value}`;
        };
        this.viewLogs = (id) => {
            window.location.href =
                "/logs?id=" +
                    id +
                    this.checkbox("stdout") +
                    this.checkbox("stderr") +
                    this.checkbox("follow") +
                    this.text("tail") +
                    this.text("since") +
                    this.text("until");
        };
        this.downloadLogs = (id) => {
            window.location.href =
                "/api/logs/download?id=" +
                    id +
                    this.checkbox("stdout") +
                    this.checkbox("stderr") +
                    this.checkbox("timestamps") +
                    this.text("tail") +
                    this.text("since") +
                    this.text("until");
        };
        this.downloadZip = () => {
            window.open("/api/logs/zip?" +
                this.checkbox("stdout") +
                this.checkbox("stderr") +
                this.checkbox("timestamps") +
                this.text("tail") +
                this.text("since") +
                this.text("until"), "download");
        };
        setText("tail", localStorage.getItem("tail") || "300");
    }
}
