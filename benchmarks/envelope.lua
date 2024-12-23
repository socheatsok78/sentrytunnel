-- example HTTP POST script which demonstrates setting the
-- HTTP method, body, and adding a header

project = 0
sentry_host = "http://127.0.0.1:3001"

wrk.method = "POST"
wrk.headers["Content-Type"] = "text/plain;charset=UTF-8"


request = function()
    sentry_dsn = sentry_host .. "/" .. project
    wrk.body   = '{"event_id":"EC200C90-F09C-48A3-9A57-3CE114C1248D","sent_at":"2024-12-12T09:58:38.344Z","sdk":{"name":"sentry.javascript.vue","version":"8.26.0"},"dsn":"' .. sentry_dsn .. '"}\n{"type":"session"}\n{"sid":"d959bbc52ffe475ca8ddf328e2a1570f","init":true,"started":"2024-12-12T09:58:38.343Z","timestamp":"2024-12-12T09:58:38.344Z","status":"ok","errors":0,"attrs":{"release":"nbcp-ncs-frontend@develop","environment":"development","user_agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"}}'
    project = project + 1
    return wrk.format(nil, nil, nil, wrk.body)
end
