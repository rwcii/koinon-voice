import Foundation
import Testing
@testable import SidecarProtocol

@Test func jsonFrameRoundTrip() throws {
    let wire = try encodeMessage(Outgoing<Empty>(type: "event", id: 7, event: "first_audio"))
    #expect(wire.prefix(4) == Data([0, 0, 0, UInt8(wire.count - 4)]))
    #expect(wire[4] == UInt8(ascii: "J"))
    var reader = FrameReader()
    reader.append(wire)
    guard case .json(let body)? = try reader.next() else {
        Issue.record("no JSON frame")
        return
    }
    let m = try JSONDecoder().decode(Incoming.self, from: body)
    #expect(m.type == "event" && m.id == 7)
    #expect(try reader.next() == nil)
}

// The same bytes Go's writeAudio produces for ID 7 and samples 1, 2, 3, 4.
@Test func audioFrameMatchesGo() throws {
    let wire = try Frame.audio(id: 7, pcm: Data([1, 2, 3, 4])).encoded()
    #expect(wire == Data([0, 0, 0, 13, UInt8(ascii: "A"), 0, 0, 0, 0, 0, 0, 0, 7, 1, 2, 3, 4]))
    var reader = FrameReader()
    reader.append(wire)
    #expect(try reader.next() == .audio(id: 7, pcm: Data([1, 2, 3, 4])))
}

@Test func framesArriveInPieces() throws {
    let wire = try encodeMessage(Outgoing<Empty>(type: "result", id: 1)) + encodeMessage(Outgoing<Empty>(type: "result", id: 2))
    var reader = FrameReader()
    var ids: [UInt64] = []
    for byte in wire {
        reader.append(Data([byte]))
        while case .json(let body)? = try reader.next() {
            ids.append(try JSONDecoder().decode(Incoming.self, from: body).id ?? 0)
        }
    }
    #expect(ids == [1, 2])
}

@Test func requestFieldsDecode() throws {
    let body = Data(#"{"type":"request","id":3,"method":"summarize","deadline_ms":900,"params":{"prompt":"p"}}"#.utf8)
    let m = try JSONDecoder().decode(Incoming.self, from: body)
    #expect(m.method == "summarize" && m.deadlineMS == 900)
    struct P: Decodable { var prompt: String }
    #expect(try JSONDecoder().decode(Params<P>.self, from: body).params?.prompt == "p")
}

@Test func malformedFrames() {
    for (wire, want) in [
        (Data([0, 0, 0, 0]), FrameError.badLength(0)),
        (Data([0xff, 0xff, 0xff, 0xff]), FrameError.badLength(0xffff_ffff)),
        (Data([0, 0, 0, 1, UInt8(ascii: "Q")]), FrameError.badKind(UInt8(ascii: "Q"))),
        (Data([0, 0, 0, 2, UInt8(ascii: "A"), 1]), FrameError.truncated),
    ] {
        var reader = FrameReader()
        reader.append(wire)
        #expect(throws: want) { try reader.next() }
    }
}

@Test func helloCarriesTheVersion() throws {
    let wire = try encodeMessage(Outgoing.hello(helper: "koinon-voice-apple", version: "0.1.0", capabilities: ["speak"], info: Empty()))
    let json = String(decoding: wire.dropFirst(5), as: UTF8.self)
    #expect(json.contains(#""protocol":1"#))
    #expect(json.contains(#""capabilities":["speak"]"#))
}
