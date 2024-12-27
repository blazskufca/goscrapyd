"use strict";
const filterNodes = () => {
    const searchQuery = document.getElementById('nodeSearch').value.toLowerCase();
    const nodes = document.querySelectorAll('#nodes_list_sidebar li');

    nodes.forEach(node => {
        const nodeName = node.querySelector('a').textContent.toLowerCase();
        if (nodeName.includes(searchQuery)) {
            node.style.display = '';
        } else {
            node.style.display = 'none';
        }
    });
};
document.getElementById('nodeSearch').addEventListener('input', filterNodes);
