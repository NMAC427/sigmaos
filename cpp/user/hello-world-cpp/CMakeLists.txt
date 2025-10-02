message(STATUS "Building hello-world-cpp proc")

add_executable(hello-world-cpp
  main.cc
)

target_link_libraries(hello-world-cpp PRIVATE sigmaos_core)

target_include_directories(hello-world-cpp PRIVATE ${CMAKE_SOURCE_DIR})
