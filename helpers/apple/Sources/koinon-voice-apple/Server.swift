import Foundation
import SidecarProtocol

/// Output writes frames to stdout, one at a time.
final class Output: @unchecked Sendable {
    private let lock = NSLock()

    func send<D: Encodable>(_ m: Outgoing<D>) {
        guard let wire = try? encodeMessage(m) else { return }
        lock.lock()
        defer { lock.unlock() }
        FileHandle.standardOutput.write(wire)
    }

    func event<D: Encodable>(_ id: UInt64, _ name: String, _ data: D) {
        send(Outgoing(type: "event", id: id, event: name, data: data))
    }

    func result<D: Encodable>(_ id: UInt64, _ data: D) {
        send(Outgoing(type: "result", id: id, data: data))
    }

    func error(_ id: UInt64, _ code: String, _ message: String) {
        send(Outgoing<Empty>(type: "error", id: id, code: code, message: message))
    }
}

/// HelperError is an error for one request, sent as its code and message.
struct HelperError: Error {
    var code: String
    var message: String
}

/// Request is one request as a handler sees it: its raw message, and its input for a
/// request that takes input frames.
struct Request {
    var id: UInt64
    var body: Data
    var input: AsyncStream<Data>

    func params<P: Decodable>(_ type: P.Type) throws -> P {
        do {
            guard let p = try JSONDecoder().decode(Params<P>.self, from: body).params else {
                throw HelperError(code: "bad_request", message: "no params")
            }
            return p
        } catch let e as HelperError {
            throw e
        } catch {
            throw HelperError(code: "bad_request", message: "\(error)")
        }
    }
}

/// serve reads frames from stdin until it closes and runs each request in its own task.
func serve(handlers: [String: (Request, Output) async throws -> Void], info: some Encodable) async {
    let out = Output()
    out.send(Outgoing.hello(helper: "koinon-voice-apple", version: helperVersion, capabilities: handlers.keys.sorted(), info: info))

    var inputs: [UInt64: AsyncStream<Data>.Continuation] = [:]
    var tasks: [Task<Void, Never>] = []
    for await chunk in stdinChunks() {
        frames.append(chunk)
        while true {
            let frame: Frame?
            do {
                frame = try frames.next()
            } catch {
                FileHandle.standardError.write(Data("koinon-voice-apple: \(error)\n".utf8))
                exit(2)
            }
            guard case .json(let body)? = frame else {
                if frame == nil { break }
                continue // no request sends audio to this helper
            }
            guard let m = try? JSONDecoder().decode(Incoming.self, from: body), let id = m.id else { continue }
            switch m.type {
            case "request":
                let (stream, continuation) = AsyncStream<Data>.makeStream()
                inputs[id] = continuation
                let method = m.method ?? ""
                guard let handler = handlers[method] else {
                    out.error(id, "unknown_method", method)
                    continue
                }
                let request = Request(id: id, body: body, input: stream)
                tasks.append(Task {
                    do {
                        try await handler(request, out)
                    } catch let e as HelperError {
                        out.error(id, e.code, e.message)
                    } catch {
                        out.error(id, "engine_error", "\(error)")
                    }
                })
            case "input":
                if m.end == true {
                    inputs.removeValue(forKey: id)?.finish()
                } else {
                    inputs[id]?.yield(body)
                }
            case "cancel":
                inputs.removeValue(forKey: id)?.finish()
            default:
                continue
            }
        }
    }
    // stdin closed: the bridge is done. Finish what is in flight, then exit.
    for c in inputs.values { c.finish() }
    for t in tasks { await t.value }
}

nonisolated(unsafe) private var frames = FrameReader()

/// stdinChunks yields stdin as it arrives, read on a thread of its own.
func stdinChunks() -> AsyncStream<Data> {
    AsyncStream { continuation in
        Thread {
            while true {
                let data = FileHandle.standardInput.availableData
                if data.isEmpty { break }
                continuation.yield(data)
            }
            continuation.finish()
        }.start()
    }
}
