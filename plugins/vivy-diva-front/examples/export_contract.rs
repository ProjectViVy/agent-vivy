fn main() -> std::io::Result<()> {
    let root = std::path::PathBuf::from(
        std::env::args_os()
            .nth(1)
            .expect("output directory required"),
    );
    std::fs::create_dir_all(&root)?;
    std::fs::write(
        root.join("protocol.ts"),
        vivy_diva_front::protocol::typescript(),
    )?;
    std::fs::write(
        root.join("examples.json"),
        vivy_diva_front::protocol::examples(),
    )?;
    std::fs::write(
        root.join("version.json"),
        format!("{}\n", vivy_diva_front::protocol::VERSION),
    )
}
