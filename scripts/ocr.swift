import Foundation
import Vision
import AppKit

let stderr = FileHandle.standardError

guard CommandLine.arguments.count > 1 else {
    stderr.write("Usage: ocr <image-path>\n".data(using: .utf8)!)
    exit(1)
}

let imagePath = CommandLine.arguments[1]
guard let image = NSImage(contentsOfFile: imagePath),
      let cgImage = image.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
    stderr.write("Error: cannot load image\n".data(using: .utf8)!)
    exit(1)
}

let request = VNRecognizeTextRequest { request, error in
    if let error = error {
        stderr.write("Error: \(error)\n".data(using: .utf8)!)
        return
    }
    guard let observations = request.results as? [VNRecognizedTextObservation] else { return }
    let texts = observations.compactMap { $0.topCandidates(1).first?.string }
    print(texts.joined(separator: "\n"))
}

request.recognitionLevel = VNRequestTextRecognitionLevel.accurate
request.recognitionLanguages = ["zh-Hans", "en"]

let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
do {
    try handler.perform([request])
} catch {
    stderr.write("Error: \(error)\n".data(using: .utf8)!)
    exit(1)
}
