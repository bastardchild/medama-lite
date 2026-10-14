/*
 * MedamaLite client-side behaviour.
 *
 * Three small Alpine components (theme switcher, dashboard filter state and the
 * website list) plus a Chart.js renderer that re-runs after every HTMX swap.
 * No build step: this file is embedded in the Go binary and served as-is.
 */
(function () {
	'use strict';

	// ---------------------------------------------------------------- helpers

	var FILTER_PARAMS = {
		path: 'path',
		referrer: 'referrer',
		country: 'country',
		device: 'device',
		browser: 'browser',
		os: 'os',
		utmSource: 'utm_source',
		utmMedium: 'utm_medium',
		utmCampaign: 'utm_campaign'
	};

	var FILTER_LABELS = {
		path: 'Page',
		referrer: 'Referrer',
		country: 'Country',
		device: 'Device',
		browser: 'Browser',
		os: 'OS',
		utmSource: 'UTM source',
		utmMedium: 'UTM medium',
		utmCampaign: 'UTM campaign'
	};

	var EMPTY_FILTERS = {
		path: '',
		referrer: '',
		country: '',
		device: '',
		browser: '',
		os: '',
		utmSource: '',
		utmMedium: '',
		utmCampaign: ''
	};

	function readJSON(element, fallback) {
		if (!element || !element.textContent) {
			return fallback;
		}

		try {
			return JSON.parse(element.textContent);
		} catch (error) {
			return fallback;
		}
	}

	function snippetFor() {
		return '<script async src="' + window.location.origin + '/script.js"><\/script>';
	}

	// ------------------------------------------------------------------ charts

	var charts = new WeakMap();

	function renderChart(container) {
		var dataElement = container.querySelector('.medama-chart-data');
		var canvas = container.querySelector('canvas');

		if (!dataElement || !canvas || !window.Chart) {
			if (!window.Chart) {
				container.classList.add('medama-chart-unavailable');
			}

			return;
		}

		var payload = readJSON(dataElement, null);
		if (!payload) {
			return;
		}

		var existing = charts.get(container);
		if (existing) {
			existing.destroy();
			charts.delete(container);
		}

		var styles = window.getComputedStyle(document.documentElement);
		var muted = styles.getPropertyValue('--medama-text-muted').trim() || '#94a3b8';
		var border = styles.getPropertyValue('--medama-border').trim() || 'rgba(128,128,128,0.12)';

		var ctx = canvas.getContext('2d');
		var visitorGradient = ctx.createLinearGradient(0, 0, 0, 260);
		visitorGradient.addColorStop(0, 'rgba(16, 185, 129, 0.28)');
		visitorGradient.addColorStop(1, 'rgba(16, 185, 129, 0.00)');

		var pageviewGradient = ctx.createLinearGradient(0, 0, 0, 260);
		pageviewGradient.addColorStop(0, 'rgba(99, 102, 241, 0.18)');
		pageviewGradient.addColorStop(1, 'rgba(99, 102, 241, 0.00)');

		var chart = new window.Chart(ctx, {
			type: 'line',
			data: {
				labels: payload.labels || [],
				datasets: [
					{
						label: 'Visitors',
						data: payload.visitors || [],
						borderColor: '#10b981',
						backgroundColor: visitorGradient,
						fill: true,
						tension: 0.38,
						borderWidth: 2,
						pointRadius: 0,
						pointHoverRadius: 5,
						pointHoverBackgroundColor: '#10b981',
						pointHoverBorderColor: '#ffffff',
						pointHoverBorderWidth: 2
					},
					{
						label: 'Pageviews',
						data: payload.pageviews || [],
						borderColor: '#6366f1',
						backgroundColor: pageviewGradient,
						fill: true,
						tension: 0.38,
						borderWidth: 2,
						pointRadius: 0,
						pointHoverRadius: 5,
						pointHoverBackgroundColor: '#6366f1',
						pointHoverBorderColor: '#ffffff',
						pointHoverBorderWidth: 2
					}
				]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				interaction: { mode: 'index', intersect: false },
				plugins: {
					legend: {
						position: 'top',
						align: 'end',
						labels: {
							color: muted,
							boxWidth: 8,
							boxHeight: 8,
							usePointStyle: true,
							pointStyle: 'circle',
							padding: 16,
							font: { size: 12, family: 'Inter, system-ui, sans-serif', weight: '500' }
						}
					},
					tooltip: {
						mode: 'index',
						intersect: false,
						backgroundColor: 'rgba(15, 23, 42, 0.92)',
						titleColor: '#f8fafc',
						bodyColor: '#e2e8f0',
						borderColor: 'rgba(255, 255, 255, 0.12)',
						borderWidth: 1,
						padding: 10,
						boxPadding: 6,
						usePointStyle: true,
						cornerRadius: 8,
						titleFont: { size: 12, weight: '600' },
						bodyFont: { size: 12 }
					}
				},
				scales: {
					x: {
						ticks: {
							color: muted,
							maxRotation: 0,
							autoSkipPadding: 20,
							font: { size: 11, family: 'Inter, system-ui, sans-serif' }
						},
						grid: { display: false },
						border: { display: false }
					},
					y: {
						beginAtZero: true,
						ticks: {
							color: muted,
							precision: 0,
							font: { size: 11, family: 'Inter, system-ui, sans-serif' }
						},
						grid: { color: border },
						border: { display: false }
					}
				}
			}
		});

		charts.set(container, chart);
	}

	function renderCharts(root) {
		var scope = root && root.querySelectorAll ? root : document;
		scope.querySelectorAll('[data-medama-chart]').forEach(renderChart);
	}

	// --------------------------------------------------------- Alpine: theme

	// Registering must happen before Alpine initialises the DOM. app.js is
	// loaded before the Alpine bundle, so normally the "alpine:init" listener
	// fires; the immediate branch only covers the case where Alpine is already
	// present (for example when it is bundled rather than loaded from a CDN).
	function registerAlpine() {
		window.Alpine.data('medamaTheme', function () {
			return {
				theme: 'dark',
				init: function () {},
				toggle: function () {}
			};
		});

		// ------------------------------------------------- Alpine: filter state

		window.Alpine.data('analyticsFilter', function (sourceElement) {
			var initial = readJSON(sourceElement, {});

			return {
				hostname: initial.hostname || '',
				period: initial.period || '7d',
				start: initial.start || '',
				end: initial.end || '',
				utm: initial.utm || 'referrer',
				tab: initial.tab || 'browsers',
				filters: Object.assign({}, EMPTY_FILTERS, initial.filters || {}),

				get activeFilters() {
					var self = this;

					return Object.keys(this.filters)
						.filter(function (key) {
							return Boolean(self.filters[key]);
						})
						.map(function (key) {
							return {
								key: key,
								label: FILTER_LABELS[key] || key,
								value: self.filters[key]
							};
						});
				},

				panelValue: function (kind) {
					return kind === 'utm' ? this.utm : this.tab;
				},

				setPanelValue: function (kind, value) {
					if (kind === 'utm') {
						this.utm = value;
					} else {
						this.tab = value;
					}

					this.refresh();
				},

				setPeriod: function (period) {
					this.period = period;
					this.refresh();
				},

				isFilterActive: function (key, value) {
					return Boolean(key) && this.filters[key] === value;
				},

				toggleFilter: function (key, value) {
					if (!key) {
						return;
					}

					this.filters[key] = this.filters[key] === value ? '' : value;
					this.refresh();
				},

				clearFilters: function () {
					var self = this;

					Object.keys(this.filters).forEach(function (key) {
						self.filters[key] = '';
					});

					this.refresh();
				},

				queryString: function () {
					var self = this;
					var params = new window.URLSearchParams();

					params.set('period', this.period);

					if (this.period === 'custom') {
						if (this.start) {
							params.set('start', this.start);
						}

						if (this.end) {
							params.set('end', this.end);
						}
					}

					Object.keys(this.filters).forEach(function (key) {
						if (self.filters[key]) {
							params.set(FILTER_PARAMS[key] || key, self.filters[key]);
						}
					});

					params.set('utm', this.utm);
					params.set('tab', this.tab);

					return params.toString();
				},

				panelUrl: function (kind) {
					return '/ui/partials/' + encodeURIComponent(this.hostname) + '/' + kind +
						'?' + this.queryString();
				},

				refresh: function () {
					this.syncUrl();
					window.dispatchEvent(new window.CustomEvent('medama:refresh'));
				},

				syncUrl: function () {
					if (!window.history || !window.history.replaceState) {
						return;
					}

					var url = '/d/' + encodeURIComponent(this.hostname) + '?' + this.queryString();
					window.history.replaceState({}, '', url);
				}
			};
		});

		// --------------------------------------------------- Alpine: home page

		window.Alpine.data('websitesPage', function () {
			return {
				addOpen: false,

				openAdd: function () {
					this.addOpen = true;
				},

				closeAdd: function () {
					this.addOpen = false;
				},

				onCreated: function (detail) {
					var hostname = detail && detail.hostname ? detail.hostname : '';

					this.closeAdd();

					if (this.$refs.hostname) {
						this.$refs.hostname.value = '';
					}

					var errorElement = document.getElementById('add-website-error');
					if (errorElement) {
						errorElement.textContent = '';
					}

					window.Alpine.store('snippet').show(hostname, snippetFor());
				},

				onDeleted: function () {
					// The list has already been refreshed by the swap.
				}
			};
		});

		// ------------------------------------------------ Alpine: snippet store

		window.Alpine.store('snippet', {
			open: false,
			hostname: '',
			snippet: '',
			copied: false,

			show: function (hostname, snippet) {
				this.hostname = hostname || '';
				this.snippet = snippet || snippetFor();
				this.copied = false;
				this.open = true;
			},

			close: function () {
				this.open = false;
			},

			copy: function () {
				var self = this;

				function markCopied() {
					self.copied = true;
					setTimeout(function () {
						self.copied = false;
					}, 2000);
				}

				if (navigator.clipboard && navigator.clipboard.writeText) {
					navigator.clipboard.writeText(this.snippet).then(markCopied).catch(function () {
						fallbackCopy();
					});
				} else {
					fallbackCopy();
				}

				function fallbackCopy() {
					try {
						var textarea = document.createElement('textarea');
						textarea.value = self.snippet;
						textarea.style.position = 'fixed';
						textarea.style.opacity = '0';
						document.body.appendChild(textarea);
						textarea.select();
						document.execCommand('copy');
						document.body.removeChild(textarea);
						markCopied();
					} catch (e) {
						// Clipboard failure
					}
				}
			}
		});
	}

	if (window.Alpine) {
		registerAlpine();
	} else {
		document.addEventListener('alpine:init', registerAlpine);
	}

	// -------------------------------------------------------------- HTMX hooks

	// Swap error responses too, so a failing panel can show its inline message
	// instead of silently keeping stale content.
	document.addEventListener('htmx:beforeSwap', function (event) {
		var xhr = event.detail && event.detail.xhr;

		if (xhr && xhr.status >= 400 && xhr.status < 600) {
			event.detail.shouldSwap = true;
			event.detail.isError = false;
		}
	});

	document.addEventListener('htmx:afterSwap', function (event) {
		renderCharts(event.target);
	});

	var pending = 0;

	document.addEventListener('htmx:beforeRequest', function () {
		pending += 1;
		document.body.classList.add('medama-loading');
	});

	document.addEventListener('htmx:afterRequest', function () {
		pending = Math.max(0, pending - 1);

		if (pending === 0) {
			document.body.classList.remove('medama-loading');
		}
	});

	// Initial paint: the dashboard is rendered server-side, so the chart needs
	// to be drawn once the DOM is ready.
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', function () {
			renderCharts(document);
		});
	} else {
		renderCharts(document);
	}
})();
