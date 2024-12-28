document.addEventListener('DOMContentLoaded', function() {
    const accordionItems = document.querySelectorAll('[data-accordion-target]');
    const errorFields = document.querySelectorAll('.text-red-600, .text-red-500');

    accordionItems.forEach(item => {
        const target = document.querySelector(item.getAttribute('data-accordion-target'));
        const hasError = target.querySelector('.text-red-600, .text-red-500');

        if (hasError) {
            item.setAttribute('aria-expanded', 'true');
            target.classList.remove('hidden');
        } else {
            item.setAttribute('aria-expanded', 'false');
            target.classList.add('hidden');
        }

        item.addEventListener('click', () => {
            const expanded = item.getAttribute('aria-expanded') === 'true' || false;
            item.setAttribute('aria-expanded', !expanded);
            target.classList.toggle('hidden');
        });
    });
});
