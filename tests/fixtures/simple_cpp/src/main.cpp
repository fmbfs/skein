#include "pipeline.hpp"

using namespace demo;

int main() {
    Pipeline pipeline;
    RawBuffer raw{0};
    pipeline.processFrame(raw);  // call site 1
    pipeline.processFrame(raw);  // call site 2
    return raw.size;
}
