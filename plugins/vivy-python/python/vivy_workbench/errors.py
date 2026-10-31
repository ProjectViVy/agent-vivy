class McpToolError(RuntimeError):
    """The complete server error is available as result; no call is retried."""
    def __init__(self, result):
        self.result = result
        super().__init__('MCP tool returned isError=true; inspect exception.result')
