"""Real MCP pagination, slow calls and process-environment fixture."""
import asyncio
import os
import sys
from mcp.server.lowlevel import Server
from mcp.server.stdio import stdio_server
from mcp.types import ListToolsRequest, ListToolsResult, Tool, CallToolResult, TextContent

server = Server('pages')
mode = sys.argv[1]


@server.list_tools()
async def listing(request: ListToolsRequest) -> ListToolsResult:
    if mode == 'slow-open':
        await asyncio.sleep(60)
    cursor = request.params.cursor if request.params else None
    return ListToolsResult(tools=[Tool(name='second' if cursor else 'first', inputSchema={'type': 'object'})],
                           nextCursor='next' if not cursor or mode == 'repeat' else None)


@server.call_tool()
async def calling(name, arguments):
    if arguments.get('slow'):
        await asyncio.sleep(60)
    return CallToolResult(content=[TextContent(type='text', text='done')],
                          structuredContent={'secret_present': 'RIG_SECRET_FIXTURE' in os.environ,
                                             'selected': os.environ.get('SELECTED_FIXTURE'), 'pid': os.getpid()})


async def main():
    async with stdio_server() as (read, write):
        await server.run(read, write, server.create_initialization_options())


asyncio.run(main())
