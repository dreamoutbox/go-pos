// Global POS Application JavaScript

function filterTable() {
    const input = document.getElementById('tableSearchInput');
    if (!input) return;
    const filter = input.value.toLowerCase();
    const table = document.querySelector('.table tbody');
    if (!table) return;
    const rows = table.getElementsByTagName('tr');

    for (let i = 0; i < rows.length; i++) {
        if (rows[i].cells.length <= 1) continue;
        const text = rows[i].textContent || rows[i].innerText;
        if (text.toLowerCase().indexOf(filter) > -1) {
            rows[i].style.display = "";
        } else {
            rows[i].style.display = "none";
        }
    }
}
