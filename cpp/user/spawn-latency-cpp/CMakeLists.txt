message(STATUS "Building spawn-latency-cpp proc")

add_executable(spawn-latency-cpp
  main.cc
)

target_link_libraries(spawn-latency-cpp PRIVATE sigmaos_core)

target_include_directories(spawn-latency-cpp PRIVATE ${CMAKE_SOURCE_DIR})
