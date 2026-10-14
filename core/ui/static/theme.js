/*
 * Permanent Dark Theme bootstrap.
 *
 * Ensures data-theme="dark" is enforced before first paint without inline scripts.
 */
(function () {
	'use strict';

	document.documentElement.setAttribute('data-theme', 'dark');

	try {
		window.localStorage.removeItem('medama-theme');
	} catch (error) {
		// localStorage can be unavailable in private browsing modes.
	}
})();
