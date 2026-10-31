"""Vivy's trusted auxiliary Python workbench (not a security sandbox)."""


async def host_request(kind, payload):
    from . import repl
    reply = await repl.host_request({'kind': kind, 'payload': payload})
    if not reply.get('ok'):
        raise RuntimeError(reply.get('error', 'host request failed'))
    return reply.get('result')
