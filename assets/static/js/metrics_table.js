class MetricsManager {
    constructor() {
        this.metricsCount = document.getElementById('metricsCount');
        this.lastUpdated = document.getElementById('lastUpdated');
        this.tableBody = document.getElementById('metricsTableBody');
        this.lastRefreshed = document.getElementById('lastRefreshed');
        this.userLocale = navigator.language || 'en-US';
    }

    async fetchMetrics() {
        try {
            const response = await fetch('/metrics/json');
            if (!response.ok) throw new Error('Network response was not ok');
            return await response.json();
        } catch (error) {
            console.error("Error fetching metrics:", error);
            return null;
        }
    }

    formatDateTime(timestamp) {
        return new Date(timestamp * 1000).toLocaleString(this.userLocale, {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false,
            timeZoneName: 'short'
        });
    }

    formatValue(value) {
        if (value instanceof Date) {
            return this.formatDateTime(value);
        }
        if (typeof value === 'object' && value !== null) {
            return JSON.stringify(value, null, 2);
        }
        return value;
    }

    createTableRow(key, value, index) {
        const formattedValue = this.formatValue(value);
        const isObject = typeof value === 'object' && value !== null;
        const rowClass = index % 2 === 0 ? 'bg-gray-50 dark:bg-gray-700' : 'bg-white dark:bg-gray-800';

        return `
            <tr class="${rowClass} hover:bg-gray-100 dark:hover:bg-gray-600 transition-colors duration-150 ease-in-out">
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">
                    ${key}
                </td>
                <td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 ${isObject ? '' : 'whitespace-nowrap'}">
                    ${isObject ?
            `<details class="cursor-pointer">
                            <summary class="text-indigo-600 dark:text-indigo-400 hover:text-indigo-900 dark:hover:text-indigo-300 focus:outline-none">
                                View Details
                            </summary>
                            <pre class="mt-2 p-2 bg-gray-100 dark:bg-gray-700 rounded-md overflow-x-auto text-xs">${formattedValue}</pre>
                        </details>`
            : formattedValue}
                </td>
            </tr>
        `;
    }

    flattenMetrics(data, prefix = '') {
        return Object.entries(data).reduce((acc, [key, value]) => {
            const newKey = prefix ? `${prefix}.${key}` : key;
            if (value && typeof value === 'object' && !Array.isArray(value)) {
                return [...acc, ...this.flattenMetrics(value, newKey)];
            }
            return [...acc, { key: newKey, value }];
        }, []);
    }

    updateTable(flattenedMetrics) {
        this.tableBody.innerHTML = flattenedMetrics.map(({ key, value }, index) => this.createTableRow(key, value, index)).join('');
    }

    async updateMetrics() {
        const data = await this.fetchMetrics();
        if (!data) return;

        const flattenedMetrics = this.flattenMetrics(data);
        this.metricsCount.textContent = flattenedMetrics.length;
        this.lastUpdated.textContent = this.formatDateTime(data.timestamp);
        this.updateTable(flattenedMetrics);
        this.lastRefreshed.textContent = new Date().toLocaleString(this.userLocale);
    }

    init() {
        document.documentElement.lang = this.userLocale;
        this.updateMetrics();
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const manager = new MetricsManager();
    manager.init();
});
