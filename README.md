## Verifly 😎

> Domain verification as a service

There are two ways to verify a domain.

1. Start a worker to verify a dns `txt` record. It will try for 30 minutes with exponential backoff:

```
curl -X POST https://verifly.xyz/worker -H 'Content-Type: application/json' -d @data.json
```

2. Directly call a challenge:

```
curl -X POST https://verifly.xyz/challenge -H 'Content-Type: application/json' -d @data.json
```

The payload for both endpoints is the same, the challenge being optional for the first.

```
{
  // The domain you want to verify
  "domain": "transparently.app",
  // The callback url where we post success or failure.
  "callback_url": "http://my-domain",
  // The unique challenge the user should update their txt records with. (optional)
  "challenge": "808bdb2e-8047-11e9-8aa5-9cb6d089854f"
}
```

The `callback_url` is called with the following payload:

```
{
  // The domain you want to verify
  "domain": "transparently.app",
  // The callback url where we post success or failure.
  "callback_url": "http://my-domain",
  // The unique challenge  - the user should update their txt records with. (optional)
  "challenge": "808bdb2e-8047-11e9-8aa5-9cb6d089854f"
  // Whether the txt record was found and the domain is therefore verified.
  "is_verified": true
}
```

**NOTE** This is a fun project to explore golang. If you'd like to suggest improvements on my golang or report a bug please [open an issue](issues/new).
