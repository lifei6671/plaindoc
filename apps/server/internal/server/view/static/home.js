(function () {
  const rootBody = document.body;
  const dropdownNodes = Array.from(document.querySelectorAll("[data-nav-user-dropdown='1']"))
    .filter((node) => node instanceof HTMLDetailsElement);
  const navToggleButton = document.querySelector("[data-yt-nav-toggle]");
  const searchToggleButton = document.querySelector("[data-yt-search-toggle]");
  const sidebarNode = document.querySelector("[data-yt-sidebar]");
  const sidebarCloseButton = document.querySelector("[data-yt-sidebar-close]");
  const mobileOverlayNode = document.querySelector("[data-yt-mobile-overlay]");
  const searchPanelNode = document.querySelector("[data-yt-search-panel]");
  const mobileViewportMediaQuery = window.matchMedia("(max-width: 900px)");

  let mobileNavOpen = false;
  let mobileSearchOpen = false;

  const setExpanded = (element, expanded) => {
    if (!(element instanceof HTMLElement)) {
      return;
    }
    element.setAttribute("aria-expanded", expanded ? "true" : "false");
  };

  const isMobileViewport = () => mobileViewportMediaQuery.matches;

  const applyMobileUIState = () => {
    const canUseMobileUI = isMobileViewport();
    const shouldOpenNav = canUseMobileUI && mobileNavOpen;
    const shouldOpenSearch = canUseMobileUI && mobileSearchOpen;

    if (rootBody instanceof HTMLElement) {
      rootBody.classList.toggle("yt-mobile-nav-open", shouldOpenNav);
      rootBody.classList.toggle("yt-mobile-search-open", shouldOpenSearch);
    }

    setExpanded(navToggleButton, shouldOpenNav);
    setExpanded(searchToggleButton, shouldOpenSearch);

    if (mobileOverlayNode instanceof HTMLElement) {
      mobileOverlayNode.hidden = !(canUseMobileUI && (shouldOpenNav || shouldOpenSearch));
    }
  };

  const closeMobileUI = () => {
    mobileNavOpen = false;
    mobileSearchOpen = false;
    applyMobileUIState();
  };

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
    closeMobileUI();
    closeAllDropdowns();
  });

  window.addEventListener("blur", () => {
    closeAllDropdowns();
    closeMobileUI();
  });

  if (navToggleButton instanceof HTMLButtonElement) {
    navToggleButton.addEventListener("click", () => {
      if (!isMobileViewport()) {
        return;
      }
      mobileNavOpen = !mobileNavOpen;
      if (mobileNavOpen) {
        mobileSearchOpen = false;
      }
      applyMobileUIState();
    });
  }

  if (searchToggleButton instanceof HTMLButtonElement) {
    searchToggleButton.addEventListener("click", () => {
      if (!isMobileViewport()) {
        return;
      }
      mobileSearchOpen = !mobileSearchOpen;
      if (mobileSearchOpen) {
        mobileNavOpen = false;
      }
      applyMobileUIState();
      if (mobileSearchOpen && searchPanelNode instanceof HTMLElement) {
        const inputNode = searchPanelNode.querySelector(".yt-header-search-input");
        if (inputNode instanceof HTMLInputElement) {
          inputNode.focus();
          inputNode.select();
        }
      }
    });
  }

  if (sidebarCloseButton instanceof HTMLButtonElement) {
    sidebarCloseButton.addEventListener("click", () => {
      mobileNavOpen = false;
      applyMobileUIState();
    });
  }

  if (mobileOverlayNode instanceof HTMLElement) {
    mobileOverlayNode.addEventListener("click", () => {
      closeMobileUI();
    });
  }

  if (sidebarNode instanceof HTMLElement) {
    sidebarNode.addEventListener("click", (event) => {
      if (!isMobileViewport()) {
        return;
      }
      const targetNode = event.target;
      if (!(targetNode instanceof Element)) {
        return;
      }
      if (targetNode.closest(".yt-sidebar-item")) {
        mobileNavOpen = false;
        applyMobileUIState();
      }
    });
  }

  if (mobileViewportMediaQuery && typeof mobileViewportMediaQuery.addEventListener === "function") {
    mobileViewportMediaQuery.addEventListener("change", () => {
      if (!isMobileViewport()) {
        closeMobileUI();
        return;
      }
      applyMobileUIState();
    });
  } else {
    window.addEventListener("resize", () => {
      if (!isMobileViewport()) {
        closeMobileUI();
      }
    });
  }

  applyMobileUIState();
})();
