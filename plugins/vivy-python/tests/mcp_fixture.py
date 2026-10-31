import sys
from mcp.server.fastmcp import FastMCP
from mcp.types import CallToolResult, TextContent, ToolAnnotations

server = FastMCP('fixture', host='127.0.0.1', port=int(sys.argv[1]) if len(sys.argv) > 1 else 8000)


@server.tool(title='Synthetic dataset', annotations=ToolAnnotations(readOnlyHint=True))
def dataset() -> CallToolResult:
    return CallToolResult(content=[TextContent(type='text', text='two synthetic rows')],
                          structuredContent={'rows': [{'name': 'alpha', 'value': 20}, {'name': 'beta', 'value': 22}]})


@server.tool()
def failure():
    raise ValueError('fixture failure')


if __name__ == '__main__':
    server.run(transport='streamable-http' if len(sys.argv) > 1 else 'stdio')
