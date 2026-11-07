#include "__Name__Cxx.h"

#include <numeric>

namespace __name__ {

    std::int64_t sum(std::span<const std::int32_t> values) noexcept {
        return std::accumulate(values.begin(), values.end(), std::int64_t {0});
    }

}  // namespace __name__
