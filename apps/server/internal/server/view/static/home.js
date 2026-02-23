(function () {
  const dropdownNodes = Array.from(document.querySelectorAll("[data-nav-user-dropdown='1']"))
    .filter((node) => node instanceof HTMLDetailsElement);
  if (!dropdownNodes.length) {
    return;
  }

  const closeDropdown = (dropdownNode) => {
    if (!(dropdownNode instanceof HTMLDetailsElement)) {
      return;
    }
    dropdownNode.open = false;
  };

  const closeAllDropdowns = () => {
    for (const dropdownNode of dropdownNodes) {
      closeDropdown(dropdownNode);
    }
  };

  const isEventInsideDropdown = (targetNode) => {
    if (!(targetNode instanceof Node)) {
      return false;
    }
    for (const dropdownNode of dropdownNodes) {
      if (dropdownNode.contains(targetNode)) {
        return true;
      }
    }
    return false;
  };

  document.addEventListener("pointerdown", (event) => {
    if (isEventInsideDropdown(event.target)) {
      return;
    }
    closeAllDropdowns();
  }, true);

  document.addEventListener("focusin", (event) => {
    if (isEventInsideDropdown(event.target)) {
      return;
    }
    closeAllDropdowns();
  }, true);

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return;
    }
    closeAllDropdowns();
  });

  window.addEventListener("blur", () => {
    closeAllDropdowns();
  });
})();
