#include "pipeline.hpp"

namespace demo {

void Pipeline::processFrame(RawBuffer& buf) {
    counter_ += acquire();
    buf.size = counter_;
}

int Pipeline::acquire() {
    return 1;
}

} // namespace demo
