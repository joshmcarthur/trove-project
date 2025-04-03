---
title: Installation
nav_order: 1
---

# Installing Trove

Trove is built with Deno and can be used either as a library in your Deno
applications or as a standalone binary.

## Using Trove in a Deno Project

```ts
import { Trove } from "https://deno.land/x/trove/core/mod.ts";
```

## Installing the Binary

### From Releases

Download the latest release for your platform from our
[GitHub Releases page](https://github.com/joshmcarthur/trove-project/releases).

```bash
# MacOS
curl -L https://github.com/joshmcarthur/trove-project/releases/latest/download/trove-macos -o trove
chmod +x trove

# Linux
curl -L https://github.com/joshmcarthur/trove-project/releases/latest/download/trove-linux -o trove
chmod +x trove
```

### Using Docker

```bash
docker pull ghcr.io/joshmcarthur/trove-project:latest

docker run -p 3000:3000 ghcr.io/joshmcarthur/trove-project:latest
```

## System Requirements

- For docker: None (self-contained)
- For binary: None (self-contained)
- For library: Deno 1.37 or later
- For development: Deno 1.37 or later
