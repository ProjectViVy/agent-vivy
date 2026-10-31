//! Keep provider HTTP error causes while removing the resolved credential.
use futures::StreamExt;
use rig_harness::rig_core::{
    http_client::{
        BoxedStream, Bytes, DynHttpClient, Error, HttpClientExt, LazyBody, MultipartForm, Request,
        Response, Result, StreamingResponse,
    },
    wasm_compat::WasmCompatSend,
    wire::Secret,
};
use std::collections::VecDeque;

#[derive(Clone)]
pub(crate) struct RedactingClient {
    inner: DynHttpClient,
    key: Secret,
}
impl RedactingClient {
    pub fn wrap(inner: DynHttpClient, key: &str) -> DynHttpClient {
        DynHttpClient::new(Self {
            inner,
            key: key.into(),
        })
    }
}

fn scrub(text: &str, key: &str) -> String {
    if key.is_empty() {
        return text.into();
    }
    if let Ok(mut value) = serde_json::from_str::<serde_json::Value>(text) {
        scrub_value(&mut value, key);
        return value.to_string();
    }
    let mut text = text.replace(key, "[REDACTED]");
    if let Ok(encoded) = serde_json::to_string(key) {
        text = text.replace(&encoded[1..encoded.len() - 1], "[REDACTED]");
    }
    text
}

fn scrub_value(value: &mut serde_json::Value, key: &str) {
    use serde_json::Value;
    match value {
        Value::String(text) => *text = text.replace(key, "[REDACTED]"),
        Value::Array(items) => {
            for item in items {
                scrub_value(item, key);
            }
        }
        Value::Object(fields) => {
            let original = std::mem::take(fields);
            for (name, mut value) in original {
                scrub_value(&mut value, key);
                fields.insert(name.replace(key, "[REDACTED]"), value);
            }
        }
        _ => {}
    }
}

fn json_error(bytes: Bytes, key: &str) -> Bytes {
    if key.is_empty() {
        return bytes;
    }
    if let Ok(mut value) = serde_json::from_slice::<serde_json::Value>(&bytes) {
        if value.get("error").is_some()
            || value.get("type").and_then(|v| v.as_str()) == Some("error")
        {
            scrub_value(&mut value, key);
            return Bytes::from(value.to_string());
        }
    }
    bytes
}

fn error(error: Error, key: &str) -> Error {
    match error {
        Error::InvalidStatusCodeWithDetails {
            status,
            body,
            mut headers,
        } => {
            for value in headers.values_mut() {
                if let Ok(text) = value.to_str() {
                    let clean = scrub(text, key);
                    if clean != text {
                        *value = clean
                            .parse()
                            .unwrap_or_else(|_| "[REDACTED]".parse().unwrap());
                    }
                }
            }
            Error::InvalidStatusCodeWithDetails {
                status,
                body: scrub(&body, key),
                headers,
            }
        }
        other => {
            let text = other.to_string();
            let clean = scrub(&text, key);
            if text != clean {
                Error::instance(std::io::Error::other(clean))
            } else {
                other
            }
        }
    }
}

impl HttpClientExt for RedactingClient {
    fn send<T, U>(
        &self,
        req: Request<T>,
    ) -> impl Future<Output = Result<Response<LazyBody<U>>>> + WasmCompatSend + 'static
    where
        T: Into<Bytes> + WasmCompatSend,
        U: From<Bytes> + WasmCompatSend + 'static,
    {
        let future = self.inner.send::<T, Bytes>(req);
        let key = self.key.clone();
        async move {
            let response = future.await.map_err(|e| error(e, key.expose()))?;
            Ok(response.map(move |body| -> LazyBody<U> {
                Box::pin(async move {
                    let bytes = body.await.map_err(|e| error(e, key.expose()))?;
                    Ok(U::from(json_error(bytes, key.expose())))
                })
            }))
        }
    }
    fn send_multipart<U>(
        &self,
        req: Request<MultipartForm>,
    ) -> impl Future<Output = Result<Response<LazyBody<U>>>> + WasmCompatSend + 'static
    where
        U: From<Bytes> + WasmCompatSend + 'static,
    {
        let future = self.inner.send_multipart::<Bytes>(req);
        let key = self.key.clone();
        async move {
            let response = future.await.map_err(|e| error(e, key.expose()))?;
            Ok(response.map(move |body| -> LazyBody<U> {
                Box::pin(async move {
                    let bytes = body.await.map_err(|e| error(e, key.expose()))?;
                    Ok(U::from(json_error(bytes, key.expose())))
                })
            }))
        }
    }
    async fn send_streaming<T>(&self, req: Request<T>) -> Result<StreamingResponse>
    where
        T: Into<Bytes> + WasmCompatSend,
    {
        let response = self
            .inner
            .send_streaming(req)
            .await
            .map_err(|e| error(e, self.key.expose()))?;
        if self.key.expose().is_empty() {
            return Ok(response);
        }
        let sse = response
            .headers()
            .get("content-type")
            .and_then(|v| v.to_str().ok())
            .is_some_and(|s| s.to_ascii_lowercase().contains("text/event-stream"));
        let (parts, mut source) = response.into_parts();
        let key = self.key.clone();
        let filtered: BoxedStream = if sse {
            // Rig's decoders consume data frames, not SSE event/id fields.
            // Reuse its framer for fragmented JSON and UTF-8.
            let framer = rig_harness::rig_core::http_client::framing::Framing::Sse
                .framer()
                .unwrap();
            Box::pin(futures::stream::unfold(
                (source, framer, VecDeque::<Bytes>::new(), key),
                |(mut source, mut framer, mut ready, key)| async move {
                    loop {
                        if let Some(bytes) = ready.pop_front() {
                            return Some((Ok(bytes), (source, framer, ready, key)));
                        }
                        match source.next().await {
                            Some(Ok(chunk)) => {
                                for frame in framer.push(&chunk) {
                                    let clean = json_error(
                                        Bytes::copy_from_slice(frame.as_bytes()),
                                        key.expose(),
                                    );
                                    let data =
                                        String::from_utf8_lossy(&clean).replace('\n', "\ndata: ");
                                    ready.push_back(Bytes::from(format!("data: {data}\n\n")));
                                }
                            }
                            Some(Err(failure)) => {
                                return Some((
                                    Err(error(failure, key.expose())),
                                    (source, framer, ready, key),
                                ));
                            }
                            None => return None,
                        }
                    }
                },
            ))
        } else {
            Box::pin(futures::stream::once(async move {
                let mut bytes = Vec::new();
                while let Some(chunk) = source.next().await {
                    bytes.extend_from_slice(&chunk.map_err(|e| error(e, key.expose()))?);
                }
                Ok(json_error(Bytes::from(bytes), key.expose()))
            }))
        };
        Ok(Response::from_parts(parts, filtered))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn status_and_cause_survive_redaction() {
        let key = "fixture-private-token";
        let failure = Error::non_success_with_details(
            401.try_into().unwrap(),
            Default::default(),
            format!("fixture rejection: Bearer {key}"),
        );
        let filtered = error(failure, key);
        assert_eq!(filtered.non_success_status().unwrap().as_u16(), 401);
        assert!(filtered.to_string().contains("fixture rejection"));
        assert!(!format!("{filtered:?}").contains(key));
    }
    #[test]
    fn escaped_json_errors_are_redacted_without_changing_normal_events() {
        let key = "fixture-private-token";
        let failure = Bytes::from_static(
            br#"{"type":"error","error":{"message":"\u0066ixture-private-token"}}"#,
        );
        let filtered = json_error(failure, key);
        assert!(!String::from_utf8_lossy(&filtered).contains(key));
        assert!(String::from_utf8_lossy(&filtered).contains("[REDACTED]"));
        let normal = Bytes::from_static(br#"{"choices":[{"delta":{"content":"text"}}]}"#);
        assert_eq!(json_error(normal.clone(), key), normal);
    }
}
