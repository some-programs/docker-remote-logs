// common.ts

declare var conn_url: string;

function getInputElementById(id: string): HTMLInputElement {
  return document.getElementById(id) as HTMLInputElement;
}

function isChecked(name: string): boolean {
  return getInputElementById(name).checked;
}

function uncheck(name: string) {
  getInputElementById(name).checked = false;
}

function empty(name: string) {
  getInputElementById(name).value = "";
}

function setText(name: string, text: string) {
  getInputElementById(name).value = text;
}

function getText(name: string): string {
  return getInputElementById(name).value;
}

function hasText(name: string): boolean {
  return getInputElementById(name).value !== "";
}
