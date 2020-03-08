"use strict";
// common.ts
function getInputElementById(id) {
    return document.getElementById(id);
}
function isChecked(name) {
    return getInputElementById(name).checked;
}
function uncheck(name) {
    getInputElementById(name).checked = false;
}
function empty(name) {
    getInputElementById(name).value = "";
}
function setText(name, text) {
    getInputElementById(name).value = text;
}
function getText(name) {
    return getInputElementById(name).value;
}
function hasText(name) {
    return getInputElementById(name).value !== "";
}
