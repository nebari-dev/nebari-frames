---
title: Nebari Frames
description: The registry and exchange for Frames - scoped, text-based context artifacts for AI conversations.
---

Nebari Frames is the registry and exchange for **Frames**: scoped, text-based artifacts that carry organizational context (terminology, style, goals, rules, business processes, and more) into AI conversations. A Frame composes through inheritance, is governed through role-based access control, and is consumable by any MCP-capable AI client (Claude, ChatGPT, Gemini, and others) or by Claude Code through file install.

It is the successor to [`skillsctl`](https://github.com/nebari-dev/skillsctl), which continues to ship as the Claude Code skill registry while new investment goes here.

## Why Frames

Enterprise AI adoption is gated less by model capability and more by the organizational context that turns a generic model into a specialized worker: the brand voice, the compliance constraints, the named concepts, the team norms. That context usually lives in style guides, wikis, chat history, and the heads of senior employees. Frames make it explicit, portable, inheritable, and governable - a first-class artifact an organization owns and shares on its own terms.

## Who this is for

- **Platform teams** deploying Frames on a Nebari cluster for their organization - start at [Installation](/installation/).
- **Anyone publishing or browsing Frames** with the `frames` CLI - start at [Quickstart](/quickstart/).
- **Contributors** working on Frames itself - start at [Local Development](/local-development/).

## Where to start

New to Frames? [Quickstart](/quickstart/) gets you from an installed CLI to a published, readable Frame in about five minutes. Deploying the registry itself? Go straight to [Installation](/installation/).
