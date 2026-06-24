import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeSanitize from "rehype-sanitize";

// Renders Frame prose. rehype-sanitize strips raw HTML / scripts; the default
// schema allows standard markdown elements only.
export function MarkdownView({ source }: { source: string }) {
  return (
    <div className="prose prose-sm max-w-none dark:prose-invert">
      <Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>
        {source}
      </Markdown>
    </div>
  );
}
