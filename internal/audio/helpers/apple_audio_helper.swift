import AVFoundation
import CoreMedia
import Foundation

func availableAudioDevices() -> [AVCaptureDevice] {
    let session = AVCaptureDevice.DiscoverySession(
        deviceTypes: [.microphone, .external],
        mediaType: .audio,
        position: .unspecified
    )
    return session.devices
}

struct DeviceDescriptor: Codable {
    let id: String
    let name: String
}

struct ChunkDescriptor: Codable {
    let sampleRate: Int
    let frames: [Float]
    let capturedAt: String
}

func maxInputChannels(for device: AVCaptureDevice) -> Int {
    var maxChannels = 1
    for format in device.formats {
        guard let streamDescriptionPointer = CMAudioFormatDescriptionGetStreamBasicDescription(format.formatDescription) else {
            continue
        }
        let channels = max(1, Int(streamDescriptionPointer.pointee.mChannelsPerFrame))
        if channels > maxChannels {
            maxChannels = channels
        }
    }
    return maxChannels
}

final class CaptureDelegate: NSObject, AVCaptureAudioDataOutputSampleBufferDelegate {
    private let encoder = JSONEncoder()
    private let formatter = ISO8601DateFormatter()
    private let selectedChannelIndex: Int?

    init(selectedChannelIndex: Int?) {
        self.selectedChannelIndex = selectedChannelIndex
        super.init()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    }

    func captureOutput(
        _ output: AVCaptureOutput,
        didOutput sampleBuffer: CMSampleBuffer,
        from connection: AVCaptureConnection
    ) {
        guard let format = CMSampleBufferGetFormatDescription(sampleBuffer),
              let streamDescriptionPointer = CMAudioFormatDescriptionGetStreamBasicDescription(format)
        else {
            return
        }

        let streamDescription = streamDescriptionPointer.pointee
        let sampleRate = Int(streamDescription.mSampleRate)
        let channelCount = max(1, Int(streamDescription.mChannelsPerFrame))
        let formatFlags = streamDescription.mFormatFlags
        let bitsPerChannel = max(8, Int(streamDescription.mBitsPerChannel))
        let bytesPerSample = max(1, bitsPerChannel / 8)

        var blockBuffer: CMBlockBuffer?
        var audioBufferList = AudioBufferList(
            mNumberBuffers: 1,
            mBuffers: AudioBuffer(mNumberChannels: 0, mDataByteSize: 0, mData: nil)
        )

        let status = CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(
            sampleBuffer,
            bufferListSizeNeededOut: nil,
            bufferListOut: &audioBufferList,
            bufferListSize: MemoryLayout<AudioBufferList>.size,
            blockBufferAllocator: nil,
            blockBufferMemoryAllocator: nil,
            flags: UInt32(kCMSampleBufferFlag_AudioBufferList_Assure16ByteAlignment),
            blockBufferOut: &blockBuffer
        )

        guard status == noErr else {
            return
        }

        let numberOfBuffers = Int(audioBufferList.mNumberBuffers)
        let isFloat = (formatFlags & kAudioFormatFlagIsFloat) != 0

        var channels: [[Float]] = []
        channels.reserveCapacity(numberOfBuffers)

        let convertedBuffers = UnsafeMutableAudioBufferListPointer(&audioBufferList)
        for audioBuffer in convertedBuffers {
            guard let mData = audioBuffer.mData else {
                continue
            }

            let sampleCount = Int(audioBuffer.mDataByteSize) / bytesPerSample
            if sampleCount == 0 {
                continue
            }

            let values: [Float] = withUnsafeBytes(of: mData) { _ in
                let rawPointer = UnsafeRawPointer(mData)
                if isFloat && bytesPerSample == 4 {
                    let pointer = rawPointer.bindMemory(to: Float.self, capacity: sampleCount)
                    return Array(UnsafeBufferPointer(start: pointer, count: sampleCount))
                }
                if isFloat && bytesPerSample == 8 {
                    let pointer = rawPointer.bindMemory(to: Double.self, capacity: sampleCount)
                    return Array(UnsafeBufferPointer(start: pointer, count: sampleCount)).map { Float($0) }
                }
                if bytesPerSample == 2 {
                    let pointer = rawPointer.bindMemory(to: Int16.self, capacity: sampleCount)
                    return Array(UnsafeBufferPointer(start: pointer, count: sampleCount)).map { Float($0) / 32768.0 }
                }
                if bytesPerSample == 4 {
                    let pointer = rawPointer.bindMemory(to: Int32.self, capacity: sampleCount)
                    return Array(UnsafeBufferPointer(start: pointer, count: sampleCount)).map { Float($0) / 2147483648.0 }
                }
                return []
            }

            if !values.isEmpty {
                channels.append(values)
            }
        }

        if channels.isEmpty {
            return
        }

        var frames: [Float]
        if numberOfBuffers == 1 && channelCount > 1 {
            let interleaved = channels[0]
            let frameCount = interleaved.count / channelCount
            if frameCount == 0 {
                return
            }
            frames = Array(repeating: 0, count: frameCount)
            if let selectedChannelIndex, selectedChannelIndex >= 0, selectedChannelIndex < channelCount {
                for frameIndex in 0..<frameCount {
                    frames[frameIndex] = interleaved[(frameIndex * channelCount) + selectedChannelIndex]
                }
            } else {
                for frameIndex in 0..<frameCount {
                    var sum: Float = 0
                    for channelIndex in 0..<channelCount {
                        sum += interleaved[(frameIndex * channelCount) + channelIndex]
                    }
                    frames[frameIndex] = sum / Float(channelCount)
                }
            }
        } else if channels.count > 1 {
            let frameCount = channels.map { $0.count }.min() ?? 0
            if frameCount == 0 {
                return
            }
            if let selectedChannelIndex, selectedChannelIndex >= 0, selectedChannelIndex < channels.count {
                frames = channels[selectedChannelIndex]
            } else {
                frames = Array(repeating: 0, count: frameCount)
                for frameIndex in 0..<frameCount {
                    var sum: Float = 0
                    for channel in channels {
                        sum += channel[frameIndex]
                    }
                    frames[frameIndex] = sum / Float(channels.count)
                }
            }
        } else {
            frames = channels[0]
        }

        let chunk = ChunkDescriptor(sampleRate: sampleRate, frames: frames, capturedAt: formatter.string(from: Date()))
        guard let encoded = try? encoder.encode(chunk) else {
            return
        }
        FileHandle.standardOutput.write(encoded)
        FileHandle.standardOutput.write(Data([0x0A]))
    }
}

let arguments = CommandLine.arguments
guard arguments.count >= 2 else {
    fputs("usage: apple_audio_helper.swift <list|capture>\n", stderr)
    exit(1)
}

switch arguments[1] {
case "list":
    var devices: [DeviceDescriptor] = []
    for device in availableAudioDevices() {
        let channelCount = maxInputChannels(for: device)
        devices.append(DeviceDescriptor(id: device.uniqueID, name: device.localizedName))
        if channelCount > 1 {
            for channel in 1...channelCount {
                devices.append(
                    DeviceDescriptor(
                        id: "\(device.uniqueID)::ch:\(channel)",
                        name: "\(device.localizedName) CH \(channel)"
                    )
                )
            }
        }
    }
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.prettyPrinted]
    let payload = try encoder.encode(devices)
    FileHandle.standardOutput.write(payload)

case "capture":
    guard arguments.count >= 5 else {
        fputs("usage: apple_audio_helper.swift capture <device-id> <sample-rate> <frames-per-buffer> [channel-index-1-based]\n", stderr)
        exit(1)
    }

    let deviceID = arguments[2]
    let sampleRate = Int(arguments[3]) ?? 16000
    let selectedChannelIndex: Int? = {
        guard arguments.count >= 6, let channel = Int(arguments[5]), channel > 0 else {
            return nil
        }
        return channel - 1
    }()

    guard let device = availableAudioDevices().first(where: { $0.uniqueID == deviceID }) else {
        fputs("audio device not found: \(deviceID)\n", stderr)
        exit(2)
    }

    let session = AVCaptureSession()
    session.beginConfiguration()
    do {
        let input = try AVCaptureDeviceInput(device: device)
        if session.canAddInput(input) {
            session.addInput(input)
        }
    } catch {
        fputs("failed to create audio input: \(error)\n", stderr)
        exit(3)
    }

    let output = AVCaptureAudioDataOutput()
    output.audioSettings = [
        AVFormatIDKey: kAudioFormatLinearPCM,
        AVSampleRateKey: sampleRate,
        AVLinearPCMIsFloatKey: true,
        AVLinearPCMBitDepthKey: 32,
        AVLinearPCMIsNonInterleaved: false,
    ]
    let delegate = CaptureDelegate(selectedChannelIndex: selectedChannelIndex)
    let queue = DispatchQueue(label: "procom.audio.capture")
    output.setSampleBufferDelegate(delegate, queue: queue)
    if session.canAddOutput(output) {
        session.addOutput(output)
    }
    session.commitConfiguration()
    session.startRunning()
    dispatchMain()

default:
    fputs("unknown command: \(arguments[1])\n", stderr)
    exit(1)
}