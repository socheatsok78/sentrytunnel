-- example HTTP POST script which demonstrates setting the
-- HTTP method, body, and adding a header

sentry_dsn = "https://example.sentry.i0/0"
project_name = "example-project"

wrk.method = "POST"
wrk.headers["Content-Type"] = "text/plain;charset=UTF-8"

local random = math.random
local function uuid()
    local template ='xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'
    return string.gsub(template, '[xy]', function (c)
        local v = (c == 'x') and random(0, 0xf) or random(8, 0xb)
        return string.format('%x', v)
    end)
end

request = function()
    event_id = uuid()
    wrk.body   = '{"event_id":"'..event_id..'","sent_at":"2024-12-12T09:58:38.344Z","sdk":{"name":"sentry.javascript.vue","version":"8.26.0"},"dsn":"' .. sentry_dsn .. '"}\n{"type":"session"}\n{"sid":"d959bbc52ffe475ca8ddf328e2a1570f","init":true,"started":"2024-12-12T09:58:38.343Z","timestamp":"2024-12-12T09:58:38.344Z","status":"ok","errors":0,"attrs":{"release":"'..project_name..'@benchmark","environment":"benchmark","user_agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"}}'
    return wrk.format(nil, nil, nil, wrk.body)
end
