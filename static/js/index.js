function isChecked(name) {
  return document.getElementById(name).checked;
}

function uncheck(name) {
  document.getElementById(name).checked = false;
}
function empty(name) {
  document.getElementById(name).value = "";
}
function hasText(name) {
  return document.getElementById(name).value !== "";
}

function updateFields(e) {
  var el = e.target;

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

function checkbox(name) {
  if (document.getElementById(name).checked) {
    return "&" + name + "=true";
  }
  return "&" + name + "=false";
}
function text(name) {
  if (document.getElementById(name).value === "") {
    return "";
  }
  return "&" + name + "=" + document.getElementById(name).value;
}
function viewLogs(id) {
  window.open(
    "/containers?id=" +
      id +
      checkbox("stdout") +
      checkbox("stderr") +
      checkbox("follow") +
      text("tail") +
      text("since") +
      text("until"),
    "_blank"
  );
}

function downloadLogs(id) {
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
  window.location.assign(
    "/api/logs/zip?" +
      checkbox("stdout") +
      checkbox("stderr") +
      checkbox("timestamps") +
      text("tail") +
      text("since") +
      text("until")
  );
}
