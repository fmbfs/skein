#pragma once

namespace demo {

/// A buffer of raw bytes to process.
struct RawBuffer {
    int size;
};

/// Base interface for anything that processes frames.
class IProcessor {
public:
    virtual ~IProcessor() = default;
    virtual void processFrame(RawBuffer& buf) = 0;
};

/// Concrete pipeline that processes frames.
class Pipeline : public IProcessor {
public:
    void processFrame(RawBuffer& buf) override;
    int  acquire();
private:
    int counter_ = 0;
};

} // namespace demo
