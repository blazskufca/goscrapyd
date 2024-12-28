function initFlowbite() {
    if (typeof flowbite !== 'undefined') {
        flowbite.initFlowbite();
    }
}
function toggleCheckboxes(checkbox) {
    const checkboxes = document.querySelectorAll('.task-checkbox');
    checkboxes.forEach((item) => {
        item.checked = checkbox.checked;
    });
}
document.body.addEventListener('htmx:afterSwap', function(event) {
    initFlowbite();
    if (event.target.id === 'tost') {
        const bulkContainer = document.getElementById('bulk-container');
        bulkContainer.classList.remove("hidden");
    }
});
