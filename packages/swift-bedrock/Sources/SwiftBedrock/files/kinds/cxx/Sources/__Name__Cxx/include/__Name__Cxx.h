#pragma once

#include <cstdint>
#include <span>

namespace __name__ {

    /// Sums the values, using C++20 std::span.
    [[nodiscard]] std::int64_t sum(std::span<const std::int32_t> values) noexcept;

}  // namespace __name__
