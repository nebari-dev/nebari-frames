export type ProviderStatus = "available" | "coming-soon";

export interface ConnectStep {
  title: string;
  body: string; // markdown, rendered via MarkdownView
}

export interface ConnectProvider {
  id: string; // URL-safe slug; the :provider route param
  name: string;
  blurb: string;
  status: ProviderStatus;
  steps?: ConnectStep[]; // present iff status === "available"
  verifyPrompt?: string;
  lastVerified?: string; // ISO date YYYY-MM-DD
}

export const connectProviders: ConnectProvider[] = [
  {
    id: "claude",
    name: "Claude.ai",
    blurb: "Add the Hub as a custom connector in Claude.ai and use your Frames in any chat.",
    status: "available",
    lastVerified: "2026-06-25",
    verifyPrompt:
      "List the Frames available to me and summarize my organization's brand voice Frame.",
    steps: [
      {
        title: "Open connector settings",
        body: "In Claude.ai, open **Settings** and go to **Connectors**. Click **Add custom connector**.",
      },
      {
        title: "Paste the connector URL",
        body: "Paste the **Connector URL** shown above into the URL field and give the connector a name such as `Nebari Frames`. Submit to continue.",
      },
      {
        title: "Authorize access",
        body: "Claude.ai opens the Hub's sign-in page. Log in and approve the connection. Only read access to the Frames you are entitled to is granted; the connector cannot publish or change Frames.",
      },
      {
        title: "Select Frames in a chat",
        body: "Start a new chat, open the connectors / resources picker, and choose the Frames you want as context. The Hub serves each Frame as resolved markdown.",
      },
    ],
  },
  {
    id: "chatgpt",
    name: "ChatGPT",
    blurb: "Add the Hub as a custom connector in ChatGPT (Developer Mode) and use your Frames via the list_frames and get_frame tools.",
    status: "available",
    lastVerified: "2026-06-30",
    verifyPrompt:
      "List the Frames available to me and summarize my organization's brand voice Frame.",
    steps: [
      {
        title: "Enable Developer Mode",
        body: "In ChatGPT, open **Settings**, then **Connectors**, then **Advanced settings**, and turn on **Developer Mode**. Custom connectors are available on the Pro, Team, Enterprise, and Edu plans.",
      },
      {
        title: "Create the connector",
        body: "Back on **Connectors**, click **Create**. Name it `Nebari Frames`, paste the **Connector URL** above into the URL field, and set **Authentication** to **OAuth**. Submit to continue.",
      },
      {
        title: "Authorize access",
        body: "ChatGPT registers itself and opens the Hub's sign-in page. Log in and approve. Only read access to the Frames you are entitled to is granted; the connector cannot publish or change Frames.",
      },
      {
        title: "Use your Frames in a chat",
        body: "Start a chat and ask ChatGPT to use Nebari Frames. It calls the `list_frames` and `get_frame` tools to load the Frames you ask for as context.",
      },
    ],
  },
  {
    id: "gemini",
    name: "Gemini",
    blurb: "Connect through the Gemini CLI and use your Frames via the list_frames and get_frame tools.",
    status: "available",
    lastVerified: "2026-06-30",
    verifyPrompt:
      "List the Frames available to me and summarize my organization's brand voice Frame.",
    steps: [
      {
        title: "Add the Hub to the Gemini CLI",
        body: "The Gemini app has no custom MCP connector yet, so use the [Gemini CLI](https://geminicli.com). In `~/.gemini/settings.json`, add the Hub under `mcpServers`, using the **Connector URL** above as `httpUrl`:\n\n```json\n{\n  \"mcpServers\": {\n    \"nebari-frames\": { \"httpUrl\": \"PASTE_CONNECTOR_URL\" }\n  }\n}\n```",
      },
      {
        title: "Authenticate",
        body: "Run `gemini`, then `/mcp auth nebari-frames`. A browser opens the Hub's sign-in page. Log in and approve. Only read access to your entitled Frames is granted.",
      },
      {
        title: "Use your Frames",
        body: "Confirm the connection with `/mcp list` (you should see `list_frames` and `get_frame`), then ask Gemini to use Nebari Frames in a prompt.",
      },
    ],
  },
];

export function getConnectProvider(id: string): ConnectProvider | undefined {
  return connectProviders.find((p) => p.id === id);
}
