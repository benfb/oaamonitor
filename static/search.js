let currentFocus = -1;
let searchIndex = [];
let searchIndexPromise = null;
let currentResults = [];

function normalizePlayerID(value) {
    const id = Number(value);
    if (!Number.isSafeInteger(id) || id <= 0) {
        return null;
    }
    return id;
}

function navigateToPlayer(value) {
    const id = normalizePlayerID(value);
    if (id === null) {
        return;
    }
    window.location.href = `/player/${id}`;
}

function loadSearchIndex() {
    if (!searchIndexPromise) {
        searchIndexPromise = fetch('/search-index.json')
            .then((response) => {
                if (!response.ok) {
                    throw new Error(`failed to load search index: ${response.status}`);
                }
                return response.json();
            })
            .then((data) => {
                searchIndex = Array.isArray(data) ? data : [];
            })
            .catch((error) => {
                console.error(error);
                searchIndex = [];
            });
    }
    return searchIndexPromise;
}

async function liveSearch() {
    const searchInput = document.getElementById('nav-player-search');
    const resultsDiv = document.getElementById('search-results');
    const query = searchInput.value.trim().toLowerCase();

    if (query.length === 0) {
        resultsDiv.innerHTML = '';
        resultsDiv.style.display = 'none';
        currentFocus = -1;
        currentResults = [];
        return;
    }

    await loadSearchIndex();

    const filtered = searchIndex.filter((player) =>
        typeof player.name === 'string' &&
        normalizePlayerID(player.id) !== null &&
        player.name.toLowerCase().includes(query)
    );
    currentResults = filtered;

    resultsDiv.style.display = 'block';
    resultsDiv.innerHTML = '';

    filtered.forEach((player, index) => {
        const playerDiv = document.createElement('div');
        playerDiv.textContent = player.name;
        playerDiv.classList.add('search-result-item');
        playerDiv.setAttribute('data-index', index);
        playerDiv.onclick = () => {
            navigateToPlayer(player.id);
        };
        resultsDiv.appendChild(playerDiv);
    });

    currentFocus = -1;
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
            if (currentFocus > -1 && items[currentFocus] && currentResults[currentFocus]) {
                navigateToPlayer(currentResults[currentFocus].id);
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
