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
    blurb: "Connector setup steps are being written and verified.",
    status: "coming-soon",
  },
  {
    id: "gemini",
    name: "Gemini",
    blurb: "Connector setup steps are being written and verified.",
    status: "coming-soon",
  },
];

export function getConnectProvider(id: string): ConnectProvider | undefined {
  return connectProviders.find((p) => p.id === id);
}
