(function () {
    const container = document.getElementById('reportsContent');
    let sitesData = [];

    function render(html) {
        container.innerHTML = html;
    }

    function formatDate(ts) {
        if (!ts) return '';
        const d = new Date(ts);
        if (isNaN(d.getTime())) return ts;
        return d.toLocaleDateString('en-US', {
            year: 'numeric', month: 'short', day: 'numeric',
            hour: '2-digit', minute: '2-digit'
        });
    }

    function displayName(name) {
        return name.replace(/-/g, '.');
    }

    function reportIcon(name) {
        if (name.endsWith('.json')) return 'fa-file-code';
        return 'fa-file-alt';
    }

    function renderSites(sites) {
        if (!sites || sites.length === 0) {
            render('<div class="reports-empty"><i class="fas fa-folder-open"></i><p>No reports found yet. Run a scan to see reports here.</p></div>');
            return;
        }

        let html = '<div class="reports-list">';
        sites.forEach(function (site) {
            const count = site.scans ? site.scans.length : 0;
            const latest = site.scans && site.scans.length > 0 ? site.scans[0].timestamp : '';
            html += '<div class="report-card" data-site="' + site.name + '">' +
                '<div class="report-card-icon"><i class="fas fa-globe"></i></div>' +
                '<div class="report-card-body">' +
                '<h3>' + displayName(site.name) + '</h3>' +
                '<p>' + count + ' report' + (count !== 1 ? 's' : '') + (latest ? ' &middot; Latest: ' + formatDate(latest) : '') + '</p>' +
                '</div>' +
                '<i class="fas fa-chevron-right report-card-arrow"></i>' +
                '</div>';
        });
        html += '</div>';
        render(html);

        container.querySelectorAll('.report-card[data-site]').forEach(function (card) {
            card.addEventListener('click', function () {
                const name = card.getAttribute('data-site');
                const site = sites.find(function (s) { return s.name === name; });
                if (site) renderReports(site);
            });
        });
    }

    function renderReports(site) {
        let html = '<div class="reports-back" id="reportsBack"><i class="fas fa-arrow-left"></i> All Sites</div>';
        html += '<h2 class="reports-site-title">' + displayName(site.name) + '</h2>';

        var scans = (site.scans || []).slice().sort(function (a, b) {
            return new Date(b.timestamp) - new Date(a.timestamp);
        });

        if (scans.length === 0) {
            html += '<div class="reports-empty"><p>No reports for this site.</p></div>';
        } else {
            html += '<div class="reports-list">';
            scans.forEach(function (scan) {
                html += '<div class="report-card report-card-detail">' +
                    '<div class="report-card-body">' +
                    '<h3>' + formatDate(scan.timestamp) + '</h3>' +
                    '<div class="report-files">';
                (scan.reports || []).forEach(function (report) {
                    html += '<a href="' + report.url + '" target="_blank" class="report-file-link">' +
                        '<i class="fas ' + reportIcon(report.name) + '"></i> ' + report.name +
                        ' <i class="fas fa-external-link-alt"></i></a>';
                });
                html += '</div></div></div>';
            });
            html += '</div>';
        }

        render(html);

        document.getElementById('reportsBack').addEventListener('click', function () {
            renderSites(sitesData);
        });
    }

    // Fetch reports
    fetch('/api/reports')
        .then(function (res) {
            if (!res.ok) throw new Error('Failed to load reports');
            return res.json();
        })
        .then(function (data) {
            sitesData = data.sites || [];
            renderSites(sitesData);
        })
        .catch(function (err) {
            render('<div class="reports-empty"><i class="fas fa-exclamation-triangle"></i><p>' + err.message + '</p></div>');
        });
})();
