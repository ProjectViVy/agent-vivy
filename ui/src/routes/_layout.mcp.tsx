import { createFileRoute } from '@tanstack/react-router';
import { McpView } from '@/components/mcp/McpView';

export const Route = createFileRoute('/_layout/mcp')({
  component: () => <div className="h-full"><McpView /></div>,
});
