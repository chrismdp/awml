---
name: awml-publish
description: Publish an AWML workflow to an AWML registry, awml.io or the person's own, so others can see it drawn and versioned. Use it only when the person asks to publish, share, upload or push a workflow to a registry or to awml.io. It is optional, and nothing else needs it. Never use it to describe, check or draw a workflow; the awml skill does that, offline.
license: MIT
---

# Publish to an AWML registry

Publishing saves a workflow on an AWML registry as a new numbered version, with a page that draws its views. awml.io is the public registry, free for public workflows. Publish only when the person asks to.

## 1. Find the registry tools

Look for the registry's tools: `check`, `publish`, `get`, `list` and `search`, from an MCP server or connector for an AWML registry. In Claude Code, with the server named awml, they are `mcp__awml__check`, `mcp__awml__publish` and so on.

If you have them, use them straight away and never ask which registry: whoever connected them, perhaps an admin for the whole company, chose it already. Go to step 3.

## 2. Connect, if the tools are missing

The registry is https://awml.io, unless `AWML_REGISTRY_URL` names another: with a shell, `echo $AWML_REGISTRY_URL` shows it, and its address goes in place of `https://awml.io` below. Give the person the line for their client:

- **Claude Code:** `claude mcp add --transport http awml https://awml.io/mcp`, then run `/mcp`, choose awml and sign in.
- **Claude.ai or Claude Desktop:** in Customize, open Connectors and add a custom connector with the URL `https://awml.io/mcp`, then connect and sign in. If it asks how Claude should register, choose to register automatically.
- **Codex:** make a token at https://awml.io/settings/tokens and put it in `AWML_TOKEN`, then run `codex mcp add awml --url https://awml.io/mcp --bearer-token-env-var AWML_TOKEN`, which adds these lines to `~/.codex/config.toml`:

```toml
[mcp_servers.awml]
url = "https://awml.io/mcp"
bearer_token_env_var = "AWML_TOKEN"
```

Every call needs the person signed in, reading included, so they sign in once as they connect. The tools then appear in a new conversation.

A registry of the person's own works the same way: put its address in place of `https://awml.io`, or set `AWML_REGISTRY_URL` to it.

If they'd rather not connect their AI, publish with the `awml` command: step 5.

## 3. Ask where it goes

Ask both at once:

- **Owner and name,** as `owner/slug`, such as `acme/client-email`. The owner is the person's username or an organisation they belong to. The slug is lowercase letters, digits and single hyphens.
- **Public or private.** Public is the default, and anyone can see it. Only an organisation can keep a workflow private, and the first version decides it for every later one.

## 4. Check, then publish

For a 0.4 workflow, run `check` with the canonical file as `security` and no `platform`. For a legacy pair, pass the security and platform files in those fields. Fix every mistake it gives, and check again until it says `No mistakes.`

Then run `publish` with the owner, the slug, the same file fields and, for the first version only, `visibility`. If it refuses, say what it said and fix what you can: a mistake in a file, an owner the person can't publish to, or private for anything but an organisation.

Show the person the page it gives back, such as `https://awml.io/acme/client-email`, and the version number.

## 5. Without the tools: the awml command

With a shell, the `awml` command publishes too. If `awml --help` doesn't list `push`, install the latest with `go install github.com/chrismdp/awml/cmd/awml@latest`. The person makes a token at https://awml.io/settings/tokens, then:

```sh
awml login
awml push bakery/client-email client-email.yaml
```

The person runs `awml login` in their own terminal, so the token never passes through the conversation; it asks for the token and keeps it. `awml push` checks the files, publishes them and prints the page. Add `--private` to the first push of an organisation's private workflow. Both use `AWML_REGISTRY_URL` when it is set, and `--registry` with an address names another registry.
