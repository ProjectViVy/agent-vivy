use std::{ffi::OsString, process::Command};
use vivy_process::{credential_variables, filter_credentials};

#[test]
fn references_are_filtered_without_removing_unrelated_environment() {
    let names = credential_variables(
        r#"
[providers.instances.company]
credential = { source = "env", name = "COMPANY_ACCESS" }
[providers.instances.openai]
credential = { source = "env", name = "OPENAI_API_KEY" }
"#,
    )
    .unwrap();
    assert!(names.contains(&OsString::from("COMPANY_ACCESS")));
    assert!(names.contains(&OsString::from("OPENAI_API_KEY")));
    let mut command = Command::new("irrelevant");
    command.env("UNRELATED_SETTING", "kept");
    filter_credentials(&mut command, &names);
    for name in &names {
        assert!(command.get_envs().any(|(k, v)| k == name && v.is_none()));
    }
    assert!(
        command
            .get_envs()
            .any(|(k, v)| k == "UNRELATED_SETTING" && v.is_some())
    );
    assert!(credential_variables("[providers").is_err());
}
