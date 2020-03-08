/* global localStorage */

class IndexApp {
  constructor() {
    setText("tail", localStorage.getItem("tail") || "300");
  }

  public updateFields = (e: Event) => {
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
  };

  public checkbox = (name: string): string => {
    if (getInputElementById(name).checked) {
      return "&" + name + "=true";
    }
    return "&" + name + "=false";
  };

  public text = (name: string): string => {
    if (getInputElementById(name).value === "") {
      return "";
    }
    return `&${name}=${getInputElementById(name).value}`;
  };

  public viewLogs = (id: string) => {
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

  public downloadLogs = (id: string) => {
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

  public downloadZip = () => {
    window.open(
      "/api/logs/zip?" +
        this.checkbox("stdout") +
        this.checkbox("stderr") +
        this.checkbox("timestamps") +
        this.text("tail") +
        this.text("since") +
        this.text("until"),
      "download"
    );
  };
}
