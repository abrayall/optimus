/* Optimus - Scan Page JavaScript */

(function () {
    'use strict';

    var scanForm = document.getElementById('scanForm');
    var scanFormWrapper = document.getElementById('scanFormWrapper');
    var scanStatus = document.getElementById('scanStatus');
    var scanRunning = document.getElementById('scanRunning');
    var scanDone = document.getElementById('scanDone');
    var scanError = document.getElementById('scanError');
    var scanStatusText = document.getElementById('scanStatusText');
    var scanStatusDetail = document.getElementById('scanStatusDetail');
    var scanErrorText = document.getElementById('scanErrorText');
    var scanViewReport = document.getElementById('scanViewReport');
    var scanRetry = document.getElementById('scanRetry');
    var selectAll = document.getElementById('selectAll');
    var skillChecks = document.querySelectorAll('.skill-check');

    // Select All toggle
    if (selectAll) {
        selectAll.addEventListener('change', function () {
            skillChecks.forEach(function (cb) {
                cb.checked = selectAll.checked;
            });
        });

        // Update Select All when individual checkboxes change
        skillChecks.forEach(function (cb) {
            cb.addEventListener('change', function () {
                var allChecked = true;
                skillChecks.forEach(function (c) {
                    if (!c.checked) allChecked = false;
                });
                selectAll.checked = allChecked;
            });
        });
    }

    function setScanCookie(jobId) {
        document.cookie = 'optimus_scan_job=' + jobId + ';path=/;max-age=86400';
    }

    function getScanCookie() {
        var match = document.cookie.match('(^|;)\\s*optimus_scan_job=([^;]+)');
        return match ? match[2] : null;
    }

    function clearScanCookie() {
        document.cookie = 'optimus_scan_job=;path=/;max-age=0';
    }

    function showScanStatus() {
        scanFormWrapper.style.display = 'none';
        scanStatus.style.display = 'block';
        scanRunning.style.display = 'block';
        scanDone.style.display = 'none';
        scanError.style.display = 'none';
        scanStatusText.textContent = 'Starting scan...';
        scanStatusDetail.textContent = 'This may take a few minutes';
    }

    function showScanDone(reportURL) {
        scanRunning.style.display = 'none';
        scanDone.style.display = 'block';
        scanFormWrapper.style.display = 'none';
        scanStatus.style.display = 'block';
        scanViewReport.href = reportURL || '#';
    }

    function showScanError(message) {
        scanRunning.style.display = 'none';
        scanError.style.display = 'block';
        scanErrorText.textContent = message || 'Something went wrong';
        clearScanCookie();
    }

    function resetScanForm() {
        scanFormWrapper.style.display = 'block';
        scanStatus.style.display = 'none';
        scanForm.reset();
        // Re-check all by default
        selectAll.checked = true;
        skillChecks.forEach(function (cb) { cb.checked = true; });
        clearScanCookie();
    }

    function pollScanJob(jobId, autoOpen) {
        fetch('/api/jobs/' + jobId)
            .then(function (res) {
                if (!res.ok) {
                    clearScanCookie();
                    resetScanForm();
                    return null;
                }
                return res.json();
            })
            .then(function (job) {
                if (!job) return;
                if (job.status === 'completed') {
                    var reportURL = job.published && job.published.HTMLURL;
                    if (reportURL && autoOpen) {
                        window.open(reportURL, '_blank');
                    }
                    showScanDone(reportURL);
                } else if (job.status === 'failed') {
                    showScanError(job.error || 'The scan failed. Please try again.');
                } else {
                    if (job.status_message) {
                        scanStatusText.textContent = job.status_message;
                        scanStatusDetail.textContent = 'This may take a few minutes';
                    } else {
                        var fallback = {
                            scraping: 'Crawling site...',
                            analyzing: 'Running analysis...',
                            rendering: 'Generating report...',
                            publishing: 'Almost done...'
                        };
                        if (fallback[job.status]) {
                            scanStatusText.textContent = fallback[job.status];
                        }
                    }
                    setTimeout(function () { pollScanJob(jobId, autoOpen); }, 3000);
                }
            })
            .catch(function () {
                showScanError('Lost connection. Please try again.');
            });
    }

    // Check for in-progress job on page load
    var existingJobId = getScanCookie();
    if (existingJobId && scanFormWrapper) {
        showScanStatus();
        pollScanJob(existingJobId, false);
    }

    if (scanForm) {
        scanForm.addEventListener('submit', function (e) {
            e.preventDefault();
            var url = document.getElementById('scanUrl').value;

            // Collect checked skills
            var selectedSkills = [];
            skillChecks.forEach(function (cb) {
                if (cb.checked) selectedSkills.push(cb.value);
            });

            if (selectedSkills.length === 0) {
                alert('Please select at least one analysis.');
                return;
            }

            // If all 6 are selected, use "all"
            var skillValue = selectedSkills.length === 6 ? 'all' : selectedSkills.join(',');

            showScanStatus();

            fetch('/api/jobs', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url: url, skill: skillValue })
            })
                .then(function (res) { return res.json(); })
                .then(function (job) {
                    if (job.id) {
                        setScanCookie(job.id);
                        pollScanJob(job.id, true);
                    } else {
                        showScanError(job.error || 'Failed to start scan. Please try again.');
                    }
                })
                .catch(function () {
                    showScanError('Failed to start scan. Please try again.');
                });
        });
    }

    if (scanRetry) {
        scanRetry.addEventListener('click', function () {
            resetScanForm();
        });
    }
})();
