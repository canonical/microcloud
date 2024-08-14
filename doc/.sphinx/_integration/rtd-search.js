document.addEventListener("readthedocs-addons-data-ready", function(event) {
    document.querySelector("[role='search'] input").addEventListener("focusin", function() {
        const event = new CustomEvent("readthedocs-search-show");
        document.dispatchEvent(event);
    });
});
