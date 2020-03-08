/* global localStorage */

function loadSettings() {
  setText("tail", localStorage.getItem("tail") || "300");
}

function isChecked(name: string): boolean {
  const el = document.getElementById(name) as HTMLInputElement;
  return el.checked;
}
function uncheck(name: string) {
  const el = document.getElementById(name) as HTMLInputElement;
  el.checked = false;
}

function empty(name: string) {
  (document.getElementById(name) as HTMLInputElement).value = "";
}
function setText(name: string, text: string) {
  (document.getElementById(name) as HTMLInputElement).value = text;
}
function getText(name: string): string {
  return (document.getElementById(name) as HTMLInputElement).value;
}
function hasText(name: string): boolean {
  return (document.getElementById(name) as HTMLInputElement).value !== "";
}

function updateFields(e: Event) {
  var el = e.target as Element;
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
}

function checkbox(name: string): string {
  if ((document.getElementById(name) as HTMLInputElement).checked) {
    return "&" + name + "=true";
  }
  return "&" + name + "=false";
}

function text(name: string): string {
  if ((document.getElementById(name) as HTMLInputElement).value === "") {
    return "";
  }
  return `&${name}=${(document.getElementById(name) as HTMLInputElement).value}`;
}

function viewLogs(id: string) {
  window.location.href =
    "/containers?id=" +
    id +
    checkbox("stdout") +
    checkbox("stderr") +
    checkbox("follow") +
    text("tail") +
    text("since") +
    text("until");
}

function downloadLogs(id: string) {
  window.location.href =
    "/api/logs/download?id=" +
    id +
    checkbox("stdout") +
    checkbox("stderr") +
    checkbox("timestamps") +
    text("tail") +
    text("since") +
    text("until");
}

function downloadZip() {
  window.open(
    "/api/logs/zip?" +
      checkbox("stdout") +
      checkbox("stderr") +
      checkbox("timestamps") +
      text("tail") +
      text("since") +
      text("until"),
    "download"
  );
}
