use vivy_gateway::config::{Protocol, parse, presets};

#[test]
fn configuration_and_presets_follow_compiled_protocol_features() {
    for protocol in [Protocol::Openai, Protocol::Anthropic] {
        let text = format!(
            "[providers.instances.test]\nprotocol={:?}\nbase_url='http://127.0.0.1:9'\ncredential={{source='none'}}\n[[providers.instances.test.models]]\nid='org/model'\n",
            protocol.name()
        );
        let result = parse(&text);
        assert_eq!(result.is_ok(), protocol.compiled(), "{:?}", result.err());
        assert_eq!(
            presets()
                .iter()
                .any(|p| p.format() == Some(protocol.format())),
            protocol.compiled()
        );
    }
    assert!(
        !presets()
            .iter()
            .any(|p| matches!(p.vendor(), "chatgpt" | "copilot" | "gcp.gemini"))
    );
}
