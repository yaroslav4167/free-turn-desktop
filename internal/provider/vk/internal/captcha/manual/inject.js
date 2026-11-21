(() => {
    'use strict';

    const { local: LOCAL, upstream: UPSTREAM } = window.__ftpCaptcha ?? {};
    const PROXY = '/captcha-resource/';
    const RESULT = '/local-captcha-result';
    const CHECK = 'captchaNotRobot.check';
    let finished = false;
    let pending;
    let loadFailure = '';

    function rewriteUrl(url) {
        if (!url || typeof url !== 'string') return url;
        if (!/^(?:https?:)?\/\//i.test(url)) return url;
        let parsed;
        try { parsed = new URL(url, UPSTREAM); } catch { return url; }
        if (parsed.origin === new URL(LOCAL).origin) return url;
        if (parsed.origin === new URL(UPSTREAM).origin) return LOCAL + parsed.pathname + parsed.search + parsed.hash;
        return PROXY + parsed.protocol.slice(0, -1) + '/' + parsed.host + parsed.pathname + parsed.search + parsed.hash;
    }

    function rewriteAttr(el, attr) {
        const value = el.getAttribute?.(attr);
        if (!value) return;
        const rewritten = rewriteUrl(value);
        if (rewritten !== value) el.setAttribute(attr, rewritten);
    }

    function rewriteTree(root) {
        if (!root.querySelectorAll) return;
        for (const attr of ['href', 'src', 'action']) {
            if (root.hasAttribute?.(attr)) rewriteAttr(root, attr);
            root.querySelectorAll(`[${attr}]`).forEach((el) => rewriteAttr(el, attr));
        }
    }

    // MutationObserver runs after insertion, too late to stop a script/image
    // request. Intercept resource URL assignment before the browser starts it.
    const setAttribute = Element.prototype.setAttribute;
    Element.prototype.setAttribute = function (name, value) {
        if (['src', 'href', 'action'].includes(String(name).toLowerCase())) value = rewriteUrl(String(value));
        return setAttribute.call(this, name, value);
    };
    for (const [constructor, property] of [
        [HTMLScriptElement, 'src'], [HTMLLinkElement, 'href'],
        [HTMLImageElement, 'src'], [HTMLIFrameElement, 'src'],
    ]) {
        const descriptor = Object.getOwnPropertyDescriptor(constructor.prototype, property);
        if (!descriptor?.set || !descriptor.configurable) continue;
        Object.defineProperty(constructor.prototype, property, {
            ...descriptor,
            set(value) { descriptor.set.call(this, rewriteUrl(String(value))); },
        });
    }

    const extractToken = (data) => data?.response?.success_token;

    function sendToken(token) {
        if (!token) return;
        const body = `token=${encodeURIComponent(token)}`;

        // sendBeacon переживает уход со страницы (mobile Safari)
        if (navigator.sendBeacon?.(RESULT, new Blob([body], { type: 'application/x-www-form-urlencoded' }))) {
            showDone();
            return;
        }

        fetch(RESULT, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body,
        }).then(showDone).catch(() => {
            const form = document.createElement('form');
            form.method = 'POST';
            form.action = RESULT;
            const input = document.createElement('input');
            input.type = 'hidden';
            input.name = 'token';
            input.value = token;
            form.append(input);
            document.body.append(form);
            form.submit();
        });
    }

    function showPending() {
        if (finished || pending) return;
        document.documentElement.style.colorScheme = 'dark';
        document.documentElement.style.background = '#0e0f14';
        document.body.style.background = '#0e0f14';
        document.body.style.color = '#e1e1e1';
        const banner = document.createElement('div');
        banner.id = 'ftp-captcha-status';
        banner.style.cssText = 'position:fixed;bottom:12px;left:12px;z-index:99999;max-width:min(360px,calc(100vw - 48px));padding:10px 12px;border:1px solid #535969;border-radius:8px;font:13px/1.5 system-ui,sans-serif;color:#e1e1e1;background:#1b1c24';
        const text = document.createElement('div');
        text.setAttribute('role', 'status');
        text.textContent = 'FreeTurn · Загрузка проверки VK / Loading VK verification…';
        const retry = document.createElement('button');
        retry.type = 'button';
        retry.textContent = 'Обновить страницу / Reload';
        retry.style.cssText = 'margin-top:8px;padding:6px 10px;border:1px solid #9299ab;border-radius:5px;background:#303442;color:#fff;cursor:pointer;font:inherit';
        retry.hidden = true;
        retry.addEventListener('click', () => location.reload());
        banner.append(text, retry);
        pending = {banner, text, retry};
        document.body.append(banner);
        if (loadFailure) showLoadFailure(loadFailure);
        setTimeout(() => {
            if (finished || loadFailure) return;
            const root = document.getElementById('spa_root');
            if (root && !root.childElementCount) showLoadFailure('Виджет VK не появился / VK widget did not appear.');
            else if (root?.childElementCount) banner.hidden = true;
            else {
                text.textContent = 'FreeTurn · Если проверка не появилась, обновите страницу / If verification is missing, reload the page.';
                retry.hidden = false;
            }
        }, 12000);
    }

    function showLoadFailure(message) {
        if (finished) return;
        loadFailure = message;
        if (!pending) return;
        pending.banner.hidden = false;
        pending.text.textContent = 'FreeTurn · ' + message + ' Обновите страницу; если ошибка повторится, проверьте журнал приложения / Reload; if this persists, check the app log.';
        pending.retry.hidden = false;
    }

    window.addEventListener('error', event => {
        const element = event.target;
        const resource = element?.tagName === 'SCRIPT' || (element?.tagName === 'LINK' && element.rel === 'stylesheet') ? element.src || element.href : event.filename;
        if (!resource) return;
        try {
            const url = new URL(resource, location.href);
            const proxied = url.pathname === '/generic_proxy' ? url.searchParams.get('proxy_url') : null;
            let original;
            if (url.origin === new URL(LOCAL).origin && url.pathname.startsWith(PROXY)) {
                const parts = url.pathname.slice(PROXY.length).split('/');
                const scheme = parts.shift(), host = parts.shift();
                original = new URL(scheme + '://' + host + '/' + parts.join('/'));
            } else {
                original = proxied ? new URL(proxied) : url.origin === new URL(LOCAL).origin ? new URL(url.pathname, UPSTREAM) : url;
            }
            // Optional advertising/telemetry failures must not look like a
            // failed captcha. Never display URL queries containing challenge IDs.
            if (!/(^|\.)vk\.(ru|com)$/i.test(original.hostname)) return;
            if (/^ads\./i.test(original.hostname)) return;
            showLoadFailure((event.filename ? 'Ошибка скрипта VK / VK script error: ' : 'Не загрузился ресурс VK / VK resource failed to load: ') + original.hostname + '.');
        } catch {}
    }, true);

    function showDone() {
        finished = true;
        document.body.style.background = '#000';
        document.body.innerHTML =
            '<div style="min-height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;font-family:ui-monospace,monospace;text-align:center">' +
            '<div style="font-size:16.5px;font-weight:700;color:#e1e1e1">free turn proxy</div>' +
            '<div style="font-size:13.5px;color:#818181">gg</div></div>';
        setTimeout(() => window.close(), 1000);
    }

    const xhrOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function (...args) {
        if (typeof args[1] === 'string') {
            this._origUrl = args[1];
            args[1] = rewriteUrl(args[1]);
        }
        return xhrOpen.apply(this, args);
    };

    const xhrSend = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.send = function (...args) {
        if (this._origUrl?.includes(CHECK)) {
            this.addEventListener('load', () => {
                try { sendToken(extractToken(JSON.parse(this.responseText))); } catch {}
            });
        }
        return xhrSend.apply(this, args);
    };

    const origFetch = window.fetch;
    window.fetch = function (...args) {
        const input = args[0];
        const urlStr = input instanceof Request ? input.url : input instanceof URL ? input.href : input;
        if (typeof urlStr === 'string') {
            const rewritten = rewriteUrl(urlStr);
            if (rewritten !== urlStr) args[0] = input instanceof Request ? new Request(new URL(rewritten, location.href), input) : rewritten;
        }

        const promise = origFetch.apply(this, args);
        if (typeof urlStr === 'string' && urlStr.includes(CHECK)) {
            promise
                .then((res) => res.clone().json())
                .then((data) => sendToken(extractToken(data)))
                .catch(() => {});
        }
        return promise;
    };

    document.addEventListener('submit', (event) => {
        if (event.target?.action) event.target.action = rewriteUrl(event.target.action);
    }, true);

    document.addEventListener('click', (event) => {
        const link = event.target?.closest?.('a[href]');
        if (link?.href) link.href = rewriteUrl(link.href);
    }, true);

    const formSubmit = HTMLFormElement.prototype.submit;
    HTMLFormElement.prototype.submit = function (...args) {
        if (this.action) this.action = rewriteUrl(this.action);
        return formSubmit.apply(this, args);
    };

    const windowOpen = window.open;
    window.open = function (...args) {
        if (typeof args[0] === 'string') args[0] = rewriteUrl(args[0]);
        return windowOpen.apply(this, args);
    };

    rewriteTree(document);
    new MutationObserver((mutations) => {
        for (const { type, target, attributeName, addedNodes } of mutations) {
            if (type === 'attributes') {
                rewriteAttr(target, attributeName);
                continue;
            }
            for (const node of addedNodes) {
                if (node.nodeType === 1) rewriteTree(node);
            }
        }
    }).observe(document.documentElement, {
        subtree: true,
        childList: true,
        attributes: true,
        attributeFilter: ['href', 'src', 'action'],
    });

    if (document.body) showPending();
    else document.addEventListener('DOMContentLoaded', showPending);
})();
