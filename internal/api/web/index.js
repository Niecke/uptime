/*
 * Default color theme selection 
 */
if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    document.documentElement.dataset.theme = 'dark';
}

/*
 * Main JS Code
 */
const colorMap = {
    green: 'var(--color-up)',
    red: 'var(--color-down)',
    grey: 'var(--color-unknown)'
}

/*
 * Provide the git_hash from backend
 */

const versionObj = document.querySelector("#version");
try {
    const response = await fetch("/version");
    const response_json = await response.json();
    versionObj.textContent = `${response_json.version} (${response_json.git_hash})`;
} catch (err) {
    console.error("Failed to load version.", err);
}

const main = document.querySelector("#main");
while (main.lastElementChild) {
    main.removeChild(main.lastElementChild);
}

function createCard(url, statusCode, durationMs, checkedAt, historyBarSVG, uptime) {
    let card = document.createElement("section");
    card.className = "endpoint-card";
    card.dataset.url = url;

    let header = document.createElement("div");
    header.className = "endpoint-header";
    card.appendChild(header);

    let status;
    if (statusCode >= 200 && statusCode <= 299) {
        status = "up"
    } else {
        status = "down"
    }
    let statusDot = document.createElement("span");
    statusDot.className = "status-dot " + status;
    header.appendChild(statusDot);

    let endpointURL = document.createElement("a");
    endpointURL.className = "endpoint-url";
    endpointURL.title = url;
    endpointURL.href = url;
    endpointURL.textContent = url;
    endpointURL.target = "_blank";
    header.appendChild(endpointURL);

    let endpointMeta = document.createElement("div");
    endpointMeta.className = "endpoint-meta";
    card.appendChild(endpointMeta);

    let statusCodeElem = document.createElement("span");
    statusCodeElem.className = "status-code " + status;
    statusCodeElem.textContent = statusCode;
    endpointMeta.appendChild(statusCodeElem);

    let durationElem = document.createElement("span");
    durationElem.className = "duration";
    durationElem.textContent = durationMs + "ms";
    endpointMeta.appendChild(durationElem);

    let checkedAtElem = document.createElement("span");
    checkedAtElem.className = "checked-at";
    var checkedAtDate = new Date(checkedAt);
    checkedAtElem.textContent = checkedAtDate.toISOString().replace('T', ' ').slice(0, 19);
    endpointMeta.appendChild(checkedAtElem);

    let historyBar = document.createElement("div");
    historyBar.className = "history-bar";
    historyBar.appendChild(historyBarSVG);
    card.appendChild(historyBar);

    let uptimePercentage = document.createElement("div");
    uptimePercentage.className = "uptime-percentage";
    uptimePercentage.textContent = `Uptime:${(100 * uptime).toFixed(2)}%`;
    card.appendChild(uptimePercentage);

    return card;
}

async function loadEndpoints() {
    try {
        const response = await fetch("/endpoints")
        const endpoints = await response.json()

        const svgs = await Promise.all(
            endpoints.map(endpoint => loadEndpointHistory(endpoint.id))
        )

        endpoints.forEach((endpoint, i) => {
            main.appendChild(createCard(
                endpoint.url,
                endpoint.status_code,
                endpoint.duration_ms,
                endpoint.checked_at,
                svgs[i],
                endpoint.uptime,
            ))
        })

    } catch (err) {
        console.error("Failed to load endpoints:", err)
    }
}

function createHistoryBar(buckets) {
    const totalHours = 5 * 24
    // backend already grouped by hour — index them for O(1) lookup per bar
    const byHour = new Map(buckets.map(b => [b.hour, b]))
    const hours = []
    const now = new Date()

    // generate all totalHours hour keys, oldest first
    // UTC throughout: the backend groups on UTC timestamps, and getHours/setHours
    // would land on :30 boundaries in half-hour-offset timezones
    for (let i = totalHours - 1; i >= 0; i--) {
        const d = new Date(now)
        d.setUTCHours(d.getUTCHours() - i, 0, 0, 0)
        hours.push(d.toISOString().substring(0, 13))
    }

    // for each hour, pick a color
    const colors = hours.map(key => {
        const bucket = byHour.get(key)
        if (!bucket) return colorMap['grey']
        if (bucket.failures > 0) return colorMap['red']
        return colorMap['green']
    })

    // build SVG — each hour is one rect
    // total width 100%, height 28px
    // totalHours rects with small gaps between them
    const rectWidth = 100 / totalHours; // percentage width of each bar
    const gap = 0.2;              // small gap between bars

    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', '100%');
    svg.setAttribute('height', '100%');
    svg.setAttribute('role', 'img');
    svg.setAttribute('aria-label', `Uptime history`);


    hours.forEach((key, index) => {
        const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect')

        rect.setAttribute('x', `${index * rectWidth}%`)
        rect.setAttribute('width', `${rectWidth - gap}%`)
        rect.setAttribute('height', '28')
        rect.setAttribute('fill', colors[index])
        rect.setAttribute("class", "history-entry")
        svg.appendChild(rect)
    })
    return svg

}

async function loadEndpointHistory(endpointID) {
    try {
        const response = await fetch("/endpoints/" + endpointID + "/history")
        const endpointData = await response.json()

        return createHistoryBar(endpointData.history);
    } catch (err) {
        console.error("Failed to load history:", err)
        return createHistoryBar([]) // all-grey bar instead of undefined
    }
}

loadEndpoints();

const eventSource = new EventSource("/events");

eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);

    const card = document.querySelector(`[data-url="${data.url}"]`);
    if (!card) return  // guard against unknown URLs

    let state = "down";
    if (data.status_code > 0 && data.status_code < 400) {
        state = "up"
    }
    //update glowing dot
    const dot = card.querySelector(".status-dot");
    dot.className = "status-dot " + state;

    //update http status_code
    const statusCode = card.querySelector(".status-code")
    const duration = card.querySelector(".duration")
    const checkedAt = card.querySelector(".checked-at")

    statusCode.className = "status-code " + state;
    statusCode.textContent = data.status_code;

    duration.textContent = data.duration_ms + "ms";

    checkedAt.textContent = new Date().toISOString().replace('T', ' ').slice(0, 19);
}

eventSource.onerror = (err) => {
    console.error("SSE error:", err)
}