import { createFileRoute } from '@tanstack/react-router';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { McpDemoView } from '@/components/demo/McpDemoView';
export const Route = createFileRoute('/_layout/mcp')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><McpDemoView/></div></div> });
