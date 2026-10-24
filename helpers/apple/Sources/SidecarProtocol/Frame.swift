// The sidecar protocol, as internal/sidecar/frame.go defines it: a 4-byte big-endian length,
// then one kind byte ("J" for a JSON message, "A" for audio) and the payload.
import Foundation

public let protocolVersion = 1
public let maxFrame = 16 << 20

public enum FrameError: Error, Equatable {
    case badLength(UInt32)
    case badKind(UInt8)
    case truncated
    case tooLarge(Int)
}

public enum Frame: Equatable {
    case json(Data)
    case audio(id: UInt64, pcm: Data)

    public func encoded() throws -> Data {
        var body = Data()
        switch self {
        case .json(let json):
            body.append(UInt8(ascii: "J"))
            body.append(json)
        case .audio(let id, let pcm):
            body.append(UInt8(ascii: "A"))
            withUnsafeBytes(of: id.bigEndian) { body.append(contentsOf: $0) }
            body.append(pcm)
        }
        guard body.count <= maxFrame else { throw FrameError.tooLarge(body.count) }
        var out = Data()
        withUnsafeBytes(of: UInt32(body.count).bigEndian) { out.append(contentsOf: $0) }
        out.append(body)
        return out
    }
}

/// FrameReader takes bytes as they arrive and returns whole frames.
public struct FrameReader {
    private var buffer = Data()

    public init() {}

    public mutating func append(_ data: Data) { buffer.append(data) }

    /// The next whole frame, nil while one is still arriving, or an error for a malformed one.
    public mutating func next() throws -> Frame? {
        guard buffer.count >= 4 else { return nil }
        let start = buffer.startIndex
        let n = buffer[start..<start + 4].reduce(UInt32(0)) { $0 << 8 | UInt32($1) }
        guard n > 0, n <= maxFrame else { throw FrameError.badLength(n) }
        guard buffer.count >= 4 + Int(n) else { return nil }
        let body = buffer[start + 4..<start + 4 + Int(n)]
        buffer = Data(buffer[(start + 4 + Int(n))...])
        let kind = body[body.startIndex]
        let payload = body.dropFirst()
        switch kind {
        case UInt8(ascii: "J"):
            return .json(Data(payload))
        case UInt8(ascii: "A"):
            guard payload.count >= 8 else { throw FrameError.truncated }
            let id = payload.prefix(8).reduce(UInt64(0)) { $0 << 8 | UInt64($1) }
            return .audio(id: id, pcm: Data(payload.dropFirst(8)))
        default:
            throw FrameError.badKind(kind)
        }
    }
}

/// Incoming is the part of a bridge message every request handler reads.
public struct Incoming: Decodable {
    public var type: String
    public var id: UInt64?
    public var method: String?
    public var deadlineMS: Int64?
    public var end: Bool?

    enum CodingKeys: String, CodingKey {
        case type, id, method, end
        case deadlineMS = "deadline_ms"
    }
}

/// Params decodes a message's params as one handler's type.
public struct Params<P: Decodable>: Decodable {
    public var params: P?
}

/// Outgoing is a message from the helper: hello, event, result or error.
public struct Outgoing<D: Encodable>: Encodable {
    public var type: String
    public var id: UInt64?
    public var event: String?
    public var data: D?
    public var code: String?
    public var message: String?
    // hello
    public var proto: Int?
    public var helper: String?
    public var version: String?
    public var capabilities: [String]?
    public var info: D?

    enum CodingKeys: String, CodingKey {
        case type, id, event, data, code, message, helper, version, capabilities, info
        case proto = "protocol"
    }

    public init(type: String, id: UInt64? = nil, event: String? = nil, data: D? = nil, code: String? = nil, message: String? = nil) {
        self.type = type
        self.id = id
        self.event = event
        self.data = data
        self.code = code
        self.message = message
    }

    public static func hello(helper: String, version: String, capabilities: [String], info: D) -> Outgoing<D> {
        var m = Outgoing(type: "hello")
        m.proto = protocolVersion
        m.helper = helper
        m.version = version
        m.capabilities = capabilities
        m.info = info
        return m
    }
}

/// Empty is data for a message that carries none.
public struct Empty: Codable {
    public init() {}
}

public func encodeMessage<D: Encodable>(_ m: Outgoing<D>) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys]
    return try Frame.json(encoder.encode(m)).encoded()
}
