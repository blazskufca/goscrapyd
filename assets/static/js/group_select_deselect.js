function selectAllOptions() {
    const selectElement = document.getElementById('fireNode');
    for (let option of selectElement.options) {
        option.selected = true;
    }
}

function deselectAllOptions() {
    const selectElement = document.getElementById('fireNode');
    for (let option of selectElement.options) {
        option.selected = false;
    }
}
