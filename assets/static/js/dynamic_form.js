function addArgument() {
    const container = document.getElementById('extra-arguments');
    const newArgumentDiv = document.createElement('div');
    newArgumentDiv.className = 'flex items-center space-x-2 mb-2 argument-row';

    newArgumentDiv.innerHTML = `
            <div class="flex-grow">
                <input
                    type="text"
                    class="arg-key block w-full px-3 py-2 placeholder-gray-400 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:text-white dark:border-gray-600"
                    placeholder="Argument Name"
                    required
                >
            </div>
            <div class="flex-grow">
                <input
                    type="text"
                    class="arg-value block w-full px-3 py-2 placeholder-gray-400 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500 dark:bg-gray-700 dark:text-white dark:border-gray-600"
                    placeholder="Argument Value"
                >
            </div>
            <div>
                <button
                    type="button"
                    onclick="removeArgument(this)"
                    class="px-3 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 dark:bg-red-500 dark:hover:bg-red-600"
                >
                    Remove
                </button>
            </div>
        `;

    container.appendChild(newArgumentDiv);
}

function removeArgument(button) {
    const argumentRow = button.closest('.argument-row');
    argumentRow.remove();
}

// Modify form submission to include dynamic arguments
document.getElementById('taskForm').addEventListener('submit', function(e) {
    const argumentRows = document.querySelectorAll('.argument-row');

    argumentRows.forEach(row => {
        const keyInput = row.querySelector('.arg-key');
        const valueInput = row.querySelector('.arg-value');

        if (keyInput.value.trim()) {
            const hiddenInput = document.createElement('input');
            hiddenInput.type = 'hidden';
            hiddenInput.name = keyInput.value;
            hiddenInput.value = valueInput.value || '';
            this.appendChild(hiddenInput);
        }
    });
});
