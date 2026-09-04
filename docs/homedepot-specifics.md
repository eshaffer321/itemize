# Home Depot provider

Syncs purchases from homedepot.com into Monarch, splitting each transaction by category. Wraps [`github.com/fnziman/homedepot-go`](https://github.com/fnziman/homedepot-go), which talks to the internal `/oms/customer/order/v1` endpoints homedepot.com itself uses.

**Unofficial API.** Home Depot doesn't publish this. Endpoint drift is possible; failures usually mean either the cookies expired or the schema changed.

## One-time setup: export cookies

You need a JSON file at `~/.homedepot-api/cookies.json` containing your logged-in browser cookies. Override with `HOMEDEPOT_COOKIE_FILE=/path/to/cookies.json` if you'd rather put it somewhere else.

1. Log in at [homedepot.com](https://www.homedepot.com).
2. Open DevTools (`Cmd+Option+I` on Mac, `F12` on Windows/Linux), pick the **Console** tab.
3. Paste and run:
   ```js
   copy(JSON.stringify(document.cookie.split('; ').map(c => {
     const i = c.indexOf('=');
     return { name: c.slice(0, i), value: c.slice(i + 1) };
   }), null, 2));
   console.log('Cookies copied to clipboard.');
   ```
4. Save clipboard to the file:
   ```bash
   mkdir -p ~/.homedepot-api
   pbpaste > ~/.homedepot-api/cookies.json   # macOS
   ```

The file must include a `THD_CUSTOMER` entry — that's the cookie the client decodes for the API auth token.

If the DevTools snippet doesn't grab enough (some anti-bot cookies are HTTP-only and won't appear in `document.cookie`), fall back to a browser extension like [EditThisCookie](https://www.editthiscookie.com/) to export cookies for `.homedepot.com`. See homedepot-go's README for the accepted JSON shapes.

## Usage

```bash
itemize homedepot -dry-run -days 14 -verbose    # preview
itemize homedepot -days 14                      # apply
itemize homedepot -days 14 -max 5               # cap at 5 orders
itemize homedepot -days 14 -force               # reprocess already-processed orders
```

Config via `config.yaml`:

```yaml
providers:
  homedepot:
    enabled: true
    lookback_days: 14
    max_orders: 0        # 0 = no cap
    cookie_file: ""      # empty = ~/.homedepot-api/cookies.json
```

Or env vars: `HOMEDEPOT_LOOKBACK_DAYS`, `HOMEDEPOT_MAX_ORDERS`, `HOMEDEPOT_COOKIE_FILE`.

## Both online and in-store orders are supported

Home Depot's API returns two response shapes:

- **Online** — `orderOrigin: "online"`, keyed by `orderNumber` (e.g. `WD00000000`).
- **In-store** — `orderOrigin: "instore"`, no `orderNumber`; the client uses a `hd-instore-{storeNumber}-{transactionId}` composite for dedup.

Both flow through the same categorizer + splitter downstream.

## Merchant matching

itemize matches orders to Monarch transactions by substring on the merchant name. The Home Depot provider's `DisplayName()` is `"Home Depot"`, which case-insensitively substring-matches Monarch's canonical `"THE HOME DEPOT"`. No aliases required.

If your bank labels Home Depot differently in Monarch (e.g. `"HD SUPPLY"`), file an issue and we'll add an alias mechanism.

## Known limitations

- **24-month history cap.** Home Depot's API only returns roughly the last 24 months regardless of `-days`. Older orders are unreachable.
- **No programmatic login.** homedepot.com is Akamai-protected and blocks headless browsers. Cookie replay works; automated login does not.
- **MFA step-up invalidates the cookie.** Any MFA event on your account (new-device flag, quarterly re-verify, etc.) invalidates the exported cookies. Symptom: `AuthError: home depot auth failed (status 401)`. Fix: re-export cookies from a fresh logged-in session.
- **`orginalOrderedQuantity` typo.** The upstream JSON key really is spelled that way (missing the "i"). Exposed via `LineItem.OriginalOrderedQty`.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `THD_CUSTOMER cookie not found in jar` | Cookies exported while logged out. | Log in, re-export. |
| `home depot auth failed (status 401 or 403)` | Cookies expired or MFA event invalidated the session. | Re-export from a fresh logged-in session. |
| `home depot API rate-limited the request` | Too many requests too fast. | Client already paces itself; back off and retry after a minute. |
| `home depot API returned status 5xx` | Home Depot backend issue or schema change. | Retry; if persistent, file an issue with a scrubbed reproduction. |
| Orders older than 24 months not returned | API's own cap. | Expected — cannot be worked around. |

## Attribution

Schema mapping and endpoint discovery in the underlying homedepot-go client were reverse-engineered by [joshellissh/homedepot-history](https://github.com/joshellissh/homedepot-history) (MIT).
