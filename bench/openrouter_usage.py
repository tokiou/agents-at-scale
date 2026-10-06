"""Print the OpenRouter key's cumulative spend in USD (reads OPENROUTER_API_KEY)."""

import json
import os
import urllib.request

request = urllib.request.Request(
    os.environ.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1").rstrip("/") + "/key",
    headers={"Authorization": "Bearer " + os.environ["OPENROUTER_API_KEY"]},
)
print(json.load(urllib.request.urlopen(request, timeout=30))["data"]["usage"])
