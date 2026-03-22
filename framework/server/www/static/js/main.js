/* Optimus - Main JavaScript */

(function () {
    'use strict';

    // Header scroll effect
    const header = document.getElementById('header');
    const scrollTop = document.getElementById('scrollTop');

    var headerSolid = header.hasAttribute('data-solid');

    function onScroll() {
        const scrolled = headerSolid || window.scrollY > 50;
        header.classList.toggle('scrolled', scrolled);
        scrollTop.classList.toggle('visible', window.scrollY > 400);
    }

    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();

    // Scroll to top
    scrollTop.addEventListener('click', function () {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    });

    // Mobile menu toggle
    const mobileToggle = document.getElementById('mobileToggle');
    const navLinks = document.getElementById('navLinks');

    mobileToggle.addEventListener('click', function () {
        mobileToggle.classList.toggle('active');
        navLinks.classList.toggle('active');
        document.body.style.overflow = navLinks.classList.contains('active') ? 'hidden' : '';
    });

    // Close mobile menu on link click
    navLinks.querySelectorAll('a').forEach(function (link) {
        link.addEventListener('click', function () {
            mobileToggle.classList.remove('active');
            navLinks.classList.remove('active');
            document.body.style.overflow = '';
        });
    });

    // Smooth scroll for anchor links
    document.querySelectorAll('a[href^="#"]').forEach(function (anchor) {
        anchor.addEventListener('click', function (e) {
            var target = document.querySelector(this.getAttribute('href'));
            if (target) {
                e.preventDefault();
                target.scrollIntoView({ behavior: 'smooth' });
            }
        });
    });

    // Hero audit form handling
    var auditForm = document.getElementById('auditForm');
    var auditFormWrapper = document.getElementById('auditFormWrapper');
    var auditStatus = document.getElementById('auditStatus');
    var auditRunning = document.getElementById('auditRunning');
    var auditDone = document.getElementById('auditDone');
    var auditError = document.getElementById('auditError');
    var auditStatusText = document.getElementById('auditStatusText');
    var auditStatusDetail = document.getElementById('auditStatusDetail');
    var auditErrorText = document.getElementById('auditErrorText');
    var auditViewReport = document.getElementById('auditViewReport');
    var auditRetry = document.getElementById('auditRetry');

    var statusMessages = {
        scraping: 'Scanning your website...',
        analyzing: 'Running SEO analysis...',
        rendering: 'Generating your report...',
        publishing: 'Almost done...'
    };

    function showAuditStatus() {
        auditFormWrapper.style.display = 'none';
        auditStatus.style.display = 'block';
        auditRunning.style.display = 'block';
        auditDone.style.display = 'none';
        auditError.style.display = 'none';
        auditStatusText.textContent = 'Starting your audit...';
        auditStatusDetail.textContent = 'This usually takes 1-2 minutes';
    }

    function setJobCookie(jobId) {
        document.cookie = 'optimus_job=' + jobId + ';path=/;max-age=86400';
    }

    function getJobCookie() {
        var match = document.cookie.match('(^|;)\\s*optimus_job=([^;]+)');
        return match ? match[2] : null;
    }

    function clearJobCookie() {
        document.cookie = 'optimus_job=;path=/;max-age=0';
    }

    function showAuditDone(reportURL) {
        auditRunning.style.display = 'none';
        auditDone.style.display = 'block';
        auditFormWrapper.style.display = 'none';
        auditStatus.style.display = 'block';
        auditViewReport.href = reportURL || '#';
    }

    function showAuditError(message) {
        auditRunning.style.display = 'none';
        auditError.style.display = 'block';
        auditErrorText.textContent = message || 'Something went wrong';
        clearJobCookie();
    }

    function resetAuditForm() {
        auditFormWrapper.style.display = 'block';
        auditStatus.style.display = 'none';
        auditForm.reset();
        clearJobCookie();
    }

    function pollJob(jobId, autoOpen) {
        fetch('/api/jobs/' + jobId)
            .then(function (res) {
                if (!res.ok) {
                    clearJobCookie();
                    resetAuditForm();
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
                    showAuditDone(reportURL);
                } else if (job.status === 'failed') {
                    showAuditError(job.error || 'The audit failed. Please try again.');
                } else {
                    var msg = job.status_message || statusMessages[job.status];
                    if (msg) {
                        auditStatusText.textContent = msg;
                    }
                    setTimeout(function () { pollJob(jobId, autoOpen); }, 3000);
                }
            })
            .catch(function () {
                showAuditError('Lost connection. Please try again.');
            });
    }

    // Check for in-progress or completed job on page load
    var existingJobId = getJobCookie();
    if (existingJobId && auditFormWrapper) {
        showAuditStatus();
        pollJob(existingJobId, false);
    }

    if (auditForm) {
        auditForm.addEventListener('submit', function (e) {
            e.preventDefault();
            var url = auditForm.querySelector('input[name="url"]').value;

            showAuditStatus();

            fetch('/api/jobs', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url: url, skill: 'rank' })
            })
                .then(function (res) { return res.json(); })
                .then(function (job) {
                    if (job.id) {
                        setJobCookie(job.id);
                        pollJob(job.id, true);
                    } else {
                        showAuditError('Failed to start audit. Please try again.');
                    }
                })
                .catch(function () {
                    showAuditError('Failed to start audit. Please try again.');
                });
        });
    }

    if (auditRetry) {
        auditRetry.addEventListener('click', function () {
            resetAuditForm();
        });
    }

    // FAQ accordion
    document.querySelectorAll('.faq-question').forEach(function (btn) {
        btn.addEventListener('click', function () {
            this.closest('.faq-item').classList.toggle('active');
        });
    });

    // Intersection Observer for scroll animations
    var observer = new IntersectionObserver(function (entries) {
        entries.forEach(function (entry) {
            if (entry.isIntersecting) {
                entry.target.classList.add('animate-in');
                observer.unobserve(entry.target);
            }
        });
    }, { threshold: 0.1, rootMargin: '0px 0px -50px 0px' });

    document.querySelectorAll('.service-card, .process-step, .pricing-card, .contact-wrapper').forEach(function (el) {
        el.style.opacity = '0';
        el.style.transform = 'translateY(20px)';
        el.style.transition = 'opacity 0.6s ease, transform 0.6s ease';
        observer.observe(el);
    });

    // Add animation class styles
    var style = document.createElement('style');
    style.textContent = '.animate-in { opacity: 1 !important; transform: translateY(0) !important; }';
    document.head.appendChild(style);
})();
