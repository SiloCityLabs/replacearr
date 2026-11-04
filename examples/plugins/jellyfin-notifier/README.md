# jellyfin-notifier

This plugin moved to the marketplace catalog:

https://github.com/SiloCityLabs/replacearr-plugins/tree/main/plugins/jellyfin-notifier

Install from the Replacearr UI (**Plugins → Marketplace**) or download a release asset into `REPLACEARR_PLUGIN_DIR`.

The plugin declares `Manifest.InboundWebhooks` with PathAlias `jellyfin`. The host exposes:

- `POST /api/v1/webhooks/jellyfin`
- `POST /api/v1/webhooks/jellyfin-notifier`

Auth uses `inbound_webhook_secret` (`X-Replacearr-Webhook-Secret` or `?secret=`). Jellyfin JSON parsing and `jellyfin.picked_up` emission happen inside the plugin via `handle_webhook`.
