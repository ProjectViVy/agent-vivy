use std::process::Command;

#[test]
fn missing_species_explains_the_explicit_build_step() {
    let output = Command::new(env!("CARGO_BIN_EXE_vivy"))
        .env(
            "CARGO_TARGET_DIR",
            std::env::temp_dir().join("vivy-missing-species"),
        )
        .args(["-p", "hello"])
        .output()
        .unwrap();
    assert!(!output.status.success());
    assert!(
        String::from_utf8_lossy(&output.stderr).contains("vivy build recipes/coding-agent.yaml")
    );
    assert!(output.stdout.is_empty());
}
