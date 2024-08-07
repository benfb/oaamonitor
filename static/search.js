let currentFocus = -1;

function liveSearch() {
    const query = document.getElementById('nav-player-search').value;
    if (query.length === 0) {
        document.getElementById('search-results').innerHTML = '';
        currentFocus = -1;
        return;
    }

    fetch(`/search?query=${query}`)
        .then(response => response.json())
        .then(data => {
            const resultsDiv = document.getElementById('search-results');
            resultsDiv.style.display = 'block';

            resultsDiv.innerHTML = ''; // Clear previous results

            data.results.forEach((player, index) => {
                const playerDiv = document.createElement('div');
                playerDiv.textContent = player.name;
                playerDiv.classList.add('search-result-item');
                playerDiv.setAttribute('data-id', player.id);
                playerDiv.setAttribute('data-index', index);
                playerDiv.onclick = () => {
                    window.location.href = `/player/${player.id}`;
                };
                resultsDiv.appendChild(playerDiv);
            });

            currentFocus = -1;
        })
        .catch(error => console.error('Error fetching search results:', error));
}

function addActive(items) {
    if (!items || items.length === 0) return false;
    removeActive(items);
    if (currentFocus >= items.length) currentFocus = 0;
    if (currentFocus < 0) currentFocus = items.length - 1;
    items[currentFocus].classList.add('search-result-active');
}

function removeActive(items) {
    for (let i = 0; i < items.length; i++) {
        items[i].classList.remove('search-result-active');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const searchInput = document.getElementById('nav-player-search');
    const resultsDiv = document.getElementById('search-results');

    searchInput.addEventListener('keydown', function (e) {
        let items = resultsDiv.getElementsByClassName('search-result-item');
        if (e.key === 'ArrowDown') {
            currentFocus++;
            addActive(items);
        } else if (e.key === 'ArrowUp') {
            currentFocus--;
            addActive(items);
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (currentFocus > -1 && items[currentFocus]) {
                window.location.href = `/player/${items[currentFocus].getAttribute('data-id')}`;
            }
        }
    });

    // Hide resultsDiv when clicking outside of it
    document.addEventListener('click', function(event) {
        if (!resultsDiv.contains(event.target) && event.target !== searchInput) {
            resultsDiv.style.display = 'none';
        }
    });
});

window.liveSearch = liveSearch;